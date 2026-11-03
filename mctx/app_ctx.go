package mctx

import (
	"context"
	"errors"
	"time"

	"github.com/bytedance/sonic"
)

// MaxAppCtxSize is the size, in bytes, of the largest serialized AppContext
// that StringToAppCtx parses. A typical one is a few hundred bytes.
const MaxAppCtxSize = 32 << 10

// ErrAppCtxTooLarge is returned by StringToAppCtx for input larger than
// MaxAppCtxSize.
var ErrAppCtxTooLarge = errors.New("mctx: app context is larger than MaxAppCtxSize")

// decoder keeps numbers in Extras exact: they decode to json.Number instead
// of float64, which loses precision above 2^53.
var decoder = sonic.Config{UseNumber: true}.Froze()

type key string

const appCtxKey = key("appCtxKey")

// AppContext is the request context passed between services: the calling
// user and the request's common parameters.
//
// Every field comes from the caller. Services must not treat the user, IP or
// time as verified unless they authenticated the caller themselves. An
// AppContext stored in a context is shared; do not modify it from several
// goroutines.
type AppContext struct {
	User         User             `json:"user"`
	CommonParams *ReqCommonParams `json:"common_params"`
}

// ReqCommonParams holds parameters common to every request, such as the
// request ID, the calling app, the client IP and custom extras.
type ReqCommonParams struct {
	RequestID string    `json:"request_id"`
	AppID     int32     `json:"aid"`
	AppName   string    `json:"app_name"`
	DeviceID  string    `json:"device_id,omitempty"`
	IP        string    `json:"ip,omitempty"`
	ReqTime   int64     `json:"req_time,omitempty"`
	Now       time.Time `json:"now"`

	// Extras holds custom fields. Numbers parsed by StringToAppCtx are
	// json.Number values.
	Extras map[string]any `json:"extras,omitempty"`
}

// GetExtra returns the custom field stored under key, and whether it exists.
func (r *ReqCommonParams) GetExtra(key string) (any, bool) {
	if r.Extras == nil {
		return nil, false
	}
	val, ok := r.Extras[key]
	return val, ok
}

// SetExtra stores a custom field under key.
func (r *ReqCommonParams) SetExtra(key string, value any) {
	if r.Extras == nil {
		r.Extras = make(map[string]any)
	}
	r.Extras[key] = value
}

// AppCtxFromContext returns the AppContext stored in ctx, and whether one was
// found.
func AppCtxFromContext(ctx context.Context) (*AppContext, bool) {
	appCtx, ok := ctx.Value(appCtxKey).(*AppContext)
	return appCtx, ok
}

// ContextWithAppCtx returns a copy of ctx that carries value.
func ContextWithAppCtx(ctx context.Context, value *AppContext) context.Context {
	return context.WithValue(ctx, appCtxKey, value)
}

// StringToAppCtx parses an AppContext from its JSON form. A missing
// CommonParams is replaced by an empty one, and a zero Now is set to the
// current time. Numbers in Extras keep their exact value as json.Number.
// Input larger than MaxAppCtxSize is rejected with ErrAppCtxTooLarge.
func StringToAppCtx(s string) (*AppContext, error) {
	if len(s) > MaxAppCtxSize {
		return nil, ErrAppCtxTooLarge
	}
	var appCtx AppContext
	if err := decoder.UnmarshalFromString(s, &appCtx); err != nil {
		return nil, err
	}
	if appCtx.CommonParams == nil {
		appCtx.CommonParams = &ReqCommonParams{}
	}
	if appCtx.CommonParams.Now.IsZero() {
		appCtx.CommonParams.Now = time.Now()
	}
	return &appCtx, nil
}
