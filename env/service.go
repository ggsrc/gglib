package env

import (
	"os"
	"sync"
)

// UnknownServiceName is returned by ServiceName when no service name is set.
const UnknownServiceName = "UnknownService"

var (
	serviceName    string
	setServiceName sync.Once

	serviceVersion    string
	setServiceVersion sync.Once
)

func init() {
	serviceName = os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = os.Getenv("service_name")
	}
	if serviceName == "" {
		serviceName = os.Getenv("service-name")
	}
	if serviceName == "" {
		serviceName = os.Getenv("OTEL_SERVICE_NAME")
	}
	if serviceName == "" {
		serviceName = UnknownServiceName
	}

	serviceVersion = os.Getenv("SERVICE_VERSION")
	if serviceVersion == "" {
		serviceVersion = os.Getenv("service_version")
	}
	if serviceVersion == "" {
		serviceVersion = os.Getenv("service-version")
	}
	if serviceVersion == "" {
		serviceVersion = os.Getenv("OTEL_SERVICE_VERSION")
	}
	if serviceVersion == "" {
		serviceVersion = os.Getenv("GIT_SHA")
	}
}

// SetServiceName overrides the service name read from SERVICE_NAME. Only the
// first call has an effect.
func SetServiceName(name string) {
	setServiceName.Do(func() {
		serviceName = name
	})
}

// ServiceName returns the service name from SetServiceName, SERVICE_NAME or
// OTEL_SERVICE_NAME, or UnknownServiceName.
func ServiceName() string {
	return serviceName
}

// ServiceVersion returns the service version from SetServiceVersion,
// SERVICE_VERSION, OTEL_SERVICE_VERSION or GIT_SHA, or "".
func ServiceVersion() string {
	return serviceVersion
}

// SetServiceVersion overrides the service version read from the environment.
// Only the first call has an effect.
func SetServiceVersion(version string) {
	setServiceVersion.Do(func() {
		serviceVersion = version
	})
}
