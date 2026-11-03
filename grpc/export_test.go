package grpc

import "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"

func loggingLevel(l int) logging.Level { return logging.Level(l) }
