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

// ReqCommonParams  请求通用参数
type ReqCommonParams struct {
	AppID   int32  `json:"aid"`
	AppName string `json:"app_name"`

	DeviceID         string        `json:"device_id,omitempty"`
	InstallID        int64         `json:"iid,omitempty"`
	Channel          string        `json:"channel,omitempty"`
	DevicePlatform   string        `json:"device_platform,omitempty"    query:"device_platform"`
	DeviceType       string        `json:"device_type,omitempty"        query:"device_type"`
	DeviceBrand      string        `json:"device_brand,omitempty"       query:"device_brand"`
	AC               string        `json:"ac,omitempty"                 query:"ac"`
	OSAPI            int32         `json:"os_api,omitempty"             query:"os_api"`
	OSVersion        string        `json:"os_version,omitempty"         query:"os_version"`
	VersionCode      string        `json:"version_code,omitempty"       query:"version_code"`
	VersionName      string        `json:"version_name,omitempty"       query:"version_name"`
	Language         string        `json:"language,omitempty"           query:"language"`
	Resolution       string        `json:"resolution,omitempty"         query:"resolution"`
	TimeZoneName     string        `json:"tz_name,omitempty"            query:"tz_name"`
	IP               string        `json:"ip,omitempty"`
	RemoteIP         string        `json:"remote_ip,omitempty"`
	FP               string        `json:"fp,omitempty"`
	UtmSource        string        `json:"utm_source,omitempty"`
	UtmMedium        string        `json:"utm_medium,omitempty"`
	UtmCampaign      string        `json:"utm_campaign,omitempty"`
	Idfa             string        `json:"idfa,omitempty"`
	Forwarded        string        `json:"forwarded,omitempty"`
	AppRegion        string        `json:"app_region,omitempty"`
	SysRegion        string        `json:"sys_region,omitempty"`
	AppLanguage      string        `json:"app_language,omitempty"`
	SysLanguage      string        `json:"sys_language,omitempty"`
	AppVersion       string        `json:"app_version,omitempty"`
	ReqTime          int64         `json:"req_time,omitempty"`
	Location         *LocationInfo `json:"location,omitempty"`
	FirstInstallTime int64         `json:"first_install_time,omitempty"`
	EnterFrom        string        `json:"enter_from,omitempty"`
	BuildVersion     string        `json:"build_version,omitempty"`

	Now time.Time `json:"now"`
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
