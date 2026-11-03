package env

import (
	"os"
	"sync"
)

// UnknownBuildTime is returned by BuildTime when no build time is set.
const UnknownBuildTime = "BuildTime"

var (
	buildTime    string
	setBuildTime sync.Once
)

func init() {
	buildTime = os.Getenv("BUILD_TIME")
	if buildTime == "" {
		buildTime = os.Getenv("build_time")
	}
	if buildTime == "" {
		buildTime = os.Getenv("build-time")
	}
	if buildTime == "" {
		buildTime = os.Getenv("buildTime")
	}
	if buildTime == "" {
		buildTime = UnknownBuildTime
	}
}

// SetBuildTime overrides the build time read from BUILD_TIME. Only the first
// call has an effect.
func SetBuildTime(time string) {
	setBuildTime.Do(func() {
		buildTime = time
	})
}

// BuildTime returns the build time from SetBuildTime or BUILD_TIME, or
// UnknownBuildTime.
func BuildTime() string {
	return buildTime
}
