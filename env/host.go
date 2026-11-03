package env

import (
	"os"
	"sync"
)

// UnknownHostName is returned by HostName when no host name is set.
const UnknownHostName = "UnknownHost"

var (
	hostName    string
	setHostName sync.Once
)

func init() {
	hostName = os.Getenv("HOSTNAME")
	if hostName == "" {
		hostName = os.Getenv("hostname")
	}
	if hostName == "" {
		hostName = os.Getenv("host-name")
	}
	if hostName == "" {
		hostName = os.Getenv("OTEL_HOST_NAME")
	}
	if hostName == "" {
		hostName = UnknownHostName
	}
}

// SetHostName overrides the host name read from HOSTNAME. Only the first call
// has an effect.
func SetHostName(name string) {
	setHostName.Do(func() {
		hostName = name
	})
}

// HostName returns the host name from SetHostName, HOSTNAME or
// OTEL_HOST_NAME, or UnknownHostName.
func HostName() string {
	return hostName
}
