package env

import (
	"os"
	"runtime/debug"
)

// UnknownServiceName is returned by ServiceName when no service name is set.
const UnknownServiceName = "UnknownService"

// ServiceName returns SERVICE_NAME, or OTEL_SERVICE_NAME, or
// UnknownServiceName.
func ServiceName() string {
	return firstNonEmpty(os.Getenv("SERVICE_NAME"), os.Getenv("OTEL_SERVICE_NAME"), UnknownServiceName)
}

// ServiceVersion returns SERVICE_VERSION, or GIT_SHA, or the VCS revision the
// binary was built from, or "" when none is known.
func ServiceVersion() string {
	return firstNonEmpty(os.Getenv("SERVICE_VERSION"), os.Getenv("GIT_SHA"), buildRevision())
}

// readBuildInfo is replaced in tests.
var readBuildInfo = debug.ReadBuildInfo

func buildRevision() string {
	info, ok := readBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			return s.Value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
