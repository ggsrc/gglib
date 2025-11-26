package metadata

type contextKey string

const (
	CTX_KEY_METADATA = "metadata"
	CTX_KEY_APP_CTX  = "app-ctx-bin"
)

// Context keys for use with context.WithValue
var (
	ContextKeyMetadata = contextKey(CTX_KEY_METADATA)
)
