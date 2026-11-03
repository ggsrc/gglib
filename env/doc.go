// Package env reports the deployment environment, read from ENV, and the
// service name and version. Values are read from the process environment on
// every call, so they always reflect os.Setenv calls made at start-up.
package env
