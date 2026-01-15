package mctx

import (
	"context"
	"time"

	"github.com/bytedance/sonic"
)

type key string

const appCtxKey = key("appCtxKey")

type AppContext struct {
	User         User             `json:"user"`
	CommonParams *ReqCommonParams `json:"common_params"`
}

// ReqCommonParams
type ReqCommonParams struct {
	RequestID string    `json:"request_id"`
	AppID     int32     `json:"aid"`
	AppName   string    `json:"app_name"`
	DeviceID  string    `json:"device_id,omitempty"`
	IP        string    `json:"ip,omitempty"`
	ReqTime   int64     `json:"req_time,omitempty"`
	Now       time.Time `json:"now"`

	// Extras custom fields
	Extras map[string]any `json:"extras,omitempty"`
}

// GetExtra get custom field
func (r *ReqCommonParams) GetExtra(key string) (any, bool) {
	if r.Extras == nil {
		return nil, false
	}
	val, ok := r.Extras[key]
	return val, ok
}

// SetExtra set custom field
func (r *ReqCommonParams) SetExtra(key string, value any) {
	if r.Extras == nil {
		r.Extras = make(map[string]any)
	}
	r.Extras[key] = value
}

func AppCtxFromContext(ctx context.Context) (*AppContext, bool) {
	appCtx, ok := ctx.Value(appCtxKey).(*AppContext)
	return appCtx, ok
}

func ContextWithAppCtx(ctx context.Context, value *AppContext) context.Context {
	return context.WithValue(ctx, appCtxKey, value)
}

func StringToAppCtx(s string) (*AppContext, error) {
	var appCtx AppContext
	err := sonic.UnmarshalString(s, &appCtx)
	if err != nil {
		return nil, err
	}
	if appCtx.CommonParams.Now.Unix() == 0 {
		appCtx.CommonParams.Now = time.Now()
	}
	return &appCtx, nil
}
