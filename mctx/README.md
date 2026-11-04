# mctx

```sh
go get github.com/ggsrc/gglib/mctx
```

The request context passed between services: `AppContext` says who the request
is for and carries the request's common parameters. Between services it
travels in the gRPC header `app-ctx-bin` (see
[interceptor](../interceptor#request-context-and-header-forwarding)); inside a
service it lives in the `context.Context`.

## Types

```go
type AppContext struct {
	User         User             `json:"user"`
	CommonParams *ReqCommonParams `json:"common_params"`
}

type User struct {
	AppID    int32  `json:"app_id"`
	UserID   string `json:"user_id"`
	DeviceID string `json:"device_id"`
}

type ReqCommonParams struct {
	RequestID string         `json:"request_id"`
	AppID     int32          `json:"aid"`
	AppName   string         `json:"app_name"`
	DeviceID  string         `json:"device_id,omitempty"`
	IP        string         `json:"ip,omitempty"`
	ReqTime   int64          `json:"req_time,omitempty"`
	Now       time.Time      `json:"now"`
	Extras    map[string]any `json:"extras,omitempty"` // your own fields
}
```

## API

| Name | What it does |
|---|---|
| `AppCtxFromContext(ctx) (*AppContext, bool)` | The `AppContext` in `ctx`, and whether there is one. |
| `ContextWithAppCtx(ctx, appCtx)` | A new context carrying `appCtx`. |
| `StringToAppCtx(s) (*AppContext, error)` | Parses the JSON. Input larger than `MaxAppCtxSize` (32 KiB) is rejected with `ErrAppCtxTooLarge`. A missing `common_params` becomes an empty one; a zero `now` becomes the current time. Numbers in `Extras` are decoded as `json.Number`, so large integer IDs keep their exact value. |
| `NewUser(appID, userID, deviceID)` | A `User`. |
| `User.IsLogin()` | Whether there is a user ID. |
| `User.GetAppID()` | The app ID. |
| `User.String()` | Like `app1/u42/dabc`. |
| `(*ReqCommonParams) GetExtra(key)`, `SetExtra(key, v)` | Read and write your own fields; safe when `Extras` is nil. |

## Example

```go
func (s *Service) GetProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.Profile, error) {
	appCtx, ok := mctx.AppCtxFromContext(ctx)
	if !ok || !appCtx.User.IsLogin() {
		return nil, status.Error(codes.Unauthenticated, "not signed in")
	}
	return s.profiles.Get(ctx, appCtx.User.UserID)
}
```

## Things to know

- Why `json.Number`: an integer above 2^53 decoded as a float64 turns into a
  different number, and would change on every hop.
- The `AppContext` in a context is a shared pointer. Do not modify it from
  several goroutines at once.
- Everything in it comes from the caller and is not signed; see the
  [security boundary](../interceptor#request-context-and-header-forwarding).
