// Package metadata names the gRPC metadata keys and context keys that gglib
// interceptors use.
package metadata

type contextKey string

// gRPC metadata keys. The names predate Go naming rules and are kept because
// services reference them.
const (
	// CTX_KEY_METADATA is the metadata key for a free-form metadata string.
	CTX_KEY_METADATA = "metadata" //nolint:revive,staticcheck // exported name used by services
	// CTX_KEY_APP_CTX is the metadata key for the JSON-encoded mctx.AppContext.
	CTX_KEY_APP_CTX = "app-ctx-bin" //nolint:revive,staticcheck // exported name used by services
)

// ContextKeyMetadata is the context key under which the server interceptor
// stores the CTX_KEY_METADATA value.
var ContextKeyMetadata = contextKey(CTX_KEY_METADATA)
