package env

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnv(t *testing.T) {
	tests := []struct {
		name      string
		env, otel string
		want      string
		prod      bool
		stg       bool
		beta      bool
		test      bool
		local     bool
	}{
		{name: "unset", want: Local, local: true},
		{name: "local", env: "local", want: Local, local: true},
		{name: "blank counts as unset", env: "  ", want: Local, local: true},
		{name: "prd", env: "prd", want: Production, prod: true},
		{name: "prd with spaces and capitals", env: " PRD ", want: Production, prod: true},
		{name: "prod alias", env: "prod", want: Production, prod: true},
		{name: "production alias", env: "Production", want: Production, prod: true},
		{name: "stg", env: "stg", want: Staging, stg: true},
		{name: "staging alias", env: "STAGING", want: Staging, stg: true},
		{name: "beta", env: "beta", want: Beta, beta: true},
		{name: "test", env: "test", want: Test, test: true},
		{name: "unknown counts as production", env: "Development", want: "development", prod: true},
		{name: "otel fallback", otel: "prd", want: Production, prod: true},
		{name: "otel fallback when ENV is blank", env: " ", otel: "stg", want: Staging, stg: true},
		{name: "ENV wins over otel", env: "stg", otel: "prd", want: Staging, stg: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ENV", tt.env)
			t.Setenv("OTEL_DEPLOYMENT_ENVIRONMENT", tt.otel)

			assert.Equal(t, tt.want, Env())
			assert.Equal(t, tt.prod, IsProduction(), "IsProduction")
			assert.Equal(t, tt.stg, IsStaging(), "IsStaging")
			assert.Equal(t, tt.beta, IsBeta(), "IsBeta")
			assert.Equal(t, tt.test, IsUnitTest(), "IsUnitTest")
			assert.Equal(t, tt.local, IsLocal(), "IsLocal")
		})
	}
}

func TestServiceName(t *testing.T) {
	t.Setenv("SERVICE_NAME", "")
	t.Setenv("OTEL_SERVICE_NAME", "")
	assert.Equal(t, UnknownServiceName, ServiceName())

	t.Setenv("OTEL_SERVICE_NAME", "from-otel")
	assert.Equal(t, "from-otel", ServiceName())

	t.Setenv("SERVICE_NAME", "from-service")
	assert.Equal(t, "from-service", ServiceName())
}

func TestServiceVersion(t *testing.T) {
	stubBuildInfo(t, &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs", Value: "git"},
		{Key: "vcs.revision", Value: "abc123"},
	}}, true)
	t.Setenv("SERVICE_VERSION", "")
	t.Setenv("GIT_SHA", "")
	assert.Equal(t, "abc123", ServiceVersion())

	t.Setenv("GIT_SHA", "def456")
	assert.Equal(t, "def456", ServiceVersion())

	t.Setenv("SERVICE_VERSION", "v1.2.3")
	assert.Equal(t, "v1.2.3", ServiceVersion())
}

func TestServiceVersionWithoutBuildRevision(t *testing.T) {
	t.Setenv("SERVICE_VERSION", "")
	t.Setenv("GIT_SHA", "")

	stubBuildInfo(t, &debug.BuildInfo{}, true)
	assert.Empty(t, ServiceVersion(), "build info without a VCS revision")

	stubBuildInfo(t, nil, false)
	assert.Empty(t, ServiceVersion(), "no build info")
}

func stubBuildInfo(t *testing.T, info *debug.BuildInfo, ok bool) {
	t.Helper()
	orig := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) { return info, ok }
	t.Cleanup(func() { readBuildInfo = orig })
}
