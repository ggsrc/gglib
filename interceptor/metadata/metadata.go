package metadata

type contextKey string

const (
	CTX_KEY_REQUEST_ID     = "rid"
	CTX_KEY_REQUEST_SOURCE = "request-source"
	CTX_KEY_JWT_TOKEN      = "jwt_token"
	CTX_KEY_ACCESS_TOKEN   = "access_token"
	CTX_KEY_GALXE_ID       = "galxe_id"
	CTX_KEY_ACCOUNT_ID     = "account_id"
	CTX_KEY_ACCOUNT_TYPE   = "account_type"
	CTX_KEY_ORIGIN         = "origin"
	CTX_KEY_APP_CTX        = "app-ctx-bin"
)

// Context keys for use with context.WithValue
var (
	ContextKeyRequestSource = contextKey(CTX_KEY_REQUEST_SOURCE)
	ContextKeyAccessToken   = contextKey(CTX_KEY_ACCESS_TOKEN)
	ContextKeyGalxeId       = contextKey(CTX_KEY_GALXE_ID)
	ContextKeyOrigin        = contextKey(CTX_KEY_ORIGIN)
	ContextKeyAccountId     = contextKey(CTX_KEY_ACCOUNT_ID)
	ContextKeyAccountType   = contextKey(CTX_KEY_ACCOUNT_TYPE)
)

const (
	REQUEST_SOURCE_APP  = "App"
	REQUEST_SOURCE_WEB  = "Web"
	REQUEST_SOURCE_MWEB = "MWeb"
)
