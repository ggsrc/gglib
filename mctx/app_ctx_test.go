package mctx_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ggsrc/gglib/mctx"
)

func TestStringToAppCtx(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 3, 4, 5, 6, 0, time.UTC)
	in := `{
		"user": {"app_id": 7, "user_id": "42", "device_id": "d1"},
		"common_params": {
			"request_id": "r1", "aid": 7, "app_name": "web", "device_id": "d1",
			"ip": "1.2.3.4", "req_time": 1700000000, "now": "2026-11-03T04:05:06Z",
			"extras": {"role": "admin", "api_key_id": 1234567890123456789, "ratio": 0.5}
		}
	}`

	got, err := mctx.StringToAppCtx(in)
	require.NoError(t, err)

	assert.Equal(t, mctx.User{AppID: 7, UserID: "42", DeviceID: "d1"}, got.User)
	require.NotNil(t, got.CommonParams)
	assert.Equal(t, "r1", got.CommonParams.RequestID)
	assert.Equal(t, int32(7), got.CommonParams.AppID)
	assert.Equal(t, "web", got.CommonParams.AppName)
	assert.Equal(t, "1.2.3.4", got.CommonParams.IP)
	assert.Equal(t, int64(1700000000), got.CommonParams.ReqTime)
	assert.True(t, now.Equal(got.CommonParams.Now))

	role, ok := got.CommonParams.GetExtra("role")
	assert.True(t, ok)
	assert.Equal(t, "admin", role)
}

// Numbers in Extras used to decode as float64, so IDs above 2^53 changed
// value after one hop (1234567890123456789 became ...768).
func TestStringToAppCtxKeepsLargeIntegersExact(t *testing.T) {
	t.Parallel()

	got, err := mctx.StringToAppCtx(`{"common_params":{"extras":{"a":1234567890123456789,"b":9007199254740993,"c":0.5}}}`)
	require.NoError(t, err)

	for key, want := range map[string]string{"a": "1234567890123456789", "b": "9007199254740993", "c": "0.5"} {
		v, ok := got.CommonParams.GetExtra(key)
		require.True(t, ok, key)
		n, isNumber := v.(json.Number)
		require.True(t, isNumber, "%s is %T, want json.Number", key, v)
		assert.Equal(t, want, n.String(), key)
	}
	a, _ := got.CommonParams.GetExtra("a")
	id, err := a.(json.Number).Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(1234567890123456789), id)
}

func TestStringToAppCtxDefaults(t *testing.T) {
	t.Parallel()

	before := time.Now()
	got, err := mctx.StringToAppCtx(`{"user":{"user_id":"1"}}`)
	require.NoError(t, err)
	require.NotNil(t, got.CommonParams, "missing common_params becomes empty params")
	assert.False(t, got.CommonParams.Now.Before(before), "zero Now is set to the current time")

	got, err = mctx.StringToAppCtx(`{"common_params":{"request_id":"r"}}`)
	require.NoError(t, err)
	assert.False(t, got.CommonParams.Now.IsZero())
	assert.Equal(t, "r", got.CommonParams.RequestID)
}

func TestStringToAppCtxErrors(t *testing.T) {
	t.Parallel()

	for name, in := range map[string]string{
		"not json":       "not json",
		"truncated":      `{"user":{"user_id":"1"}`,
		"wrong type":     `{"user":{"user_id":1}}`,
		"app id too big": `{"user":{"app_id":3000000000}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := mctx.StringToAppCtx(in)
			require.Error(t, err)
			assert.Nil(t, got)
		})
	}
}

func TestStringToAppCtxSizeLimit(t *testing.T) {
	t.Parallel()

	prefix, suffix := `{"common_params":{"request_id":"`, `"}}`
	fits := prefix + strings.Repeat("x", mctx.MaxAppCtxSize-len(prefix)-len(suffix)) + suffix
	require.Len(t, fits, mctx.MaxAppCtxSize)
	_, err := mctx.StringToAppCtx(fits)
	require.NoError(t, err)

	tooLarge := prefix + strings.Repeat("x", mctx.MaxAppCtxSize) + suffix
	got, err := mctx.StringToAppCtx(tooLarge)
	require.ErrorIs(t, err, mctx.ErrAppCtxTooLarge)
	assert.Nil(t, got)
}

// Fields from the wallet-login era are no longer part of User; a sender that
// still includes them is accepted and the fields are dropped.
func TestStringToAppCtxIgnoresRemovedFields(t *testing.T) {
	t.Parallel()

	got, err := mctx.StringToAppCtx(`{"user":{"user_id":"1","access_token":"secret","signature":"s","address":"a","sign_at":"t"}}`)
	require.NoError(t, err)
	assert.Equal(t, mctx.User{UserID: "1"}, got.User)

	out, err := json.Marshal(got.User)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "secret")
}

func TestExtras(t *testing.T) {
	t.Parallel()

	var p mctx.ReqCommonParams
	v, ok := p.GetExtra("missing")
	assert.False(t, ok, "nil map")
	assert.Nil(t, v)

	p.SetExtra("k", "v")
	v, ok = p.GetExtra("k")
	assert.True(t, ok)
	assert.Equal(t, "v", v)

	_, ok = p.GetExtra("other")
	assert.False(t, ok, "missing key in a non-nil map")
}

func TestContextRoundTrip(t *testing.T) {
	t.Parallel()

	got, ok := mctx.AppCtxFromContext(context.Background())
	assert.False(t, ok)
	assert.Nil(t, got)

	want := &mctx.AppContext{User: mctx.NewUser(1, "2", "3")}
	ctx := mctx.ContextWithAppCtx(context.Background(), want)
	got, ok = mctx.AppCtxFromContext(ctx)
	assert.True(t, ok)
	assert.Same(t, want, got)
}

func TestUser(t *testing.T) {
	t.Parallel()

	u := mctx.NewUser(5, "42", "dev")
	assert.Equal(t, mctx.User{AppID: 5, UserID: "42", DeviceID: "dev"}, u)
	assert.Equal(t, "app5/u42/ddev", u.String())
	assert.Equal(t, int32(5), u.GetAppID())
	assert.True(t, u.IsLogin())
	assert.False(t, mctx.User{}.IsLogin())
}

func ExampleStringToAppCtx() {
	appCtx, err := mctx.StringToAppCtx(`{"user":{"user_id":"42"},"common_params":{"extras":{"api_key_id":1234567890123456789}}}`)
	if err != nil {
		panic(err)
	}
	id, _ := appCtx.CommonParams.GetExtra("api_key_id")
	fmt.Println(appCtx.User.UserID, id)
	// Output: 42 1234567890123456789
}
