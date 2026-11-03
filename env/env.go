package env

import "os"

// IsProduction reports whether ENV is "prd".
func IsProduction() bool {
	return os.Getenv("ENV") == "prd"
}

// IsBeta reports whether ENV is "beta".
func IsBeta() bool {
	return os.Getenv("ENV") == "beta"
}

// IsStaging reports whether ENV is "stg".
func IsStaging() bool {
	return os.Getenv("ENV") == "stg"
}

// IsLocal reports whether ENV is unset or empty.
func IsLocal() bool {
	return os.Getenv("ENV") == ""
}

// IsUnitTest reports whether ENV is "test".
func IsUnitTest() bool {
	return os.Getenv("ENV") == "test"
}

// Env returns ENV, or OTEL_DEPLOYMENT_ENVIRONMENT when ENV is empty, or
// "local" when both are empty.
func Env() string {
	e := os.Getenv("ENV")
	if e == "" {
		e = os.Getenv("OTEL_DEPLOYMENT_ENVIRONMENT")
	}
	if e == "" {
		return "local"
	}
	return e
}
