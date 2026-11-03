package grpc

import (
	"context"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/rs/zerolog"
)

// interceptorLogger adapts zerolog to the go-grpc-middleware logging
// interceptor. It logs through the logger in the call's context, so lines
// carry request_id and user_id, and falls back to l when the context has
// none.
func interceptorLogger(l zerolog.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		base := zerolog.Ctx(ctx)
		if base.GetLevel() == zerolog.Disabled {
			base = &l
		}
		logger := base.With().Fields(fields).Logger()
		switch lvl {
		case logging.LevelDebug:
			logger.Debug().Msg(msg)
		case logging.LevelInfo:
			logger.Info().Msg(msg)
		case logging.LevelWarn:
			logger.Warn().Msg(msg)
		default:
			logger.Error().Msg(msg)
		}
	})
}
