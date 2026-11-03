package mctx

import (
	"context"
	"time"

	"github.com/bytedance/sonic"
)

type key string

const appCtxKey = key("appCtxKey")

// AppContext is the request context passed between services: the calling
// user and the request's common parameters.
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

	// Extras custom fields
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
// current time.
func StringToAppCtx(s string) (*AppContext, error) {
	var appCtx AppContext
	err := sonic.UnmarshalString(s, &appCtx)
	if err != nil {
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
