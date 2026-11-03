// Package mctx carries request-scoped values through a context.Context: the
// calling user, the request's common parameters and custom extras. The
// interceptor module serializes an AppContext into gRPC metadata and restores
// it on the receiving side.
package mctx
