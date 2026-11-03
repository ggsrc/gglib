package zerolog

import (
	"github.com/rs/zerolog"
)

// InitLogger sets the global level to debug or info and installs the default
// context logger configured by opts.
func InitLogger(debug bool, opts ...LoggerOption) {
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	setupLogger(opts...)
}
