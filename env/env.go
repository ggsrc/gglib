package env

import (
	"os"
	"strings"
)

// Canonical environment names returned by Env.
const (
	Production = "prd"
	Staging    = "stg"
	Beta       = "beta"
	Test       = "test"
	Local      = "local"
)

// aliases maps every accepted spelling, after trimming and lower-casing, to
// its canonical name.
var aliases = map[string]string{
	"":           Local,
	"local":      Local,
	"prd":        Production,
	"prod":       Production,
	"production": Production,
	"stg":        Staging,
	"staging":    Staging,
	"beta":       Beta,
	"test":       Test,
}

// Env returns the deployment environment, read from ENV or, when ENV is
// empty, from OTEL_DEPLOYMENT_ENVIRONMENT. Known spellings are returned in
// canonical form ("prd", "stg", "beta", "test", "local"; an empty value is
// "local"). Any other value is returned trimmed and lower-cased, and is
// treated as production by IsProduction.
func Env() string {
	raw := os.Getenv("ENV")
	if strings.TrimSpace(raw) == "" {
		raw = os.Getenv("OTEL_DEPLOYMENT_ENVIRONMENT")
	}
	v := strings.ToLower(strings.TrimSpace(raw))
	if canonical, ok := aliases[v]; ok {
		return canonical
	}
	return v
}

// IsProduction reports whether the environment is production, or is a value
// Env does not recognize. Unrecognized values count as production so that
// behavior meant for other environments, such as recording request bodies,
// never turns on by mistake.
func IsProduction() bool {
	switch Env() {
	case Staging, Beta, Test, Local:
		return false
	default:
		return true
	}
}

// IsStaging reports whether the environment is staging.
func IsStaging() bool { return Env() == Staging }

// IsBeta reports whether the environment is beta.
func IsBeta() bool { return Env() == Beta }

// IsUnitTest reports whether the environment is test.
func IsUnitTest() bool { return Env() == Test }

// IsLocal reports whether the environment is local, that is, whether ENV and
// OTEL_DEPLOYMENT_ENVIRONMENT are both empty or "local".
func IsLocal() bool { return Env() == Local }
