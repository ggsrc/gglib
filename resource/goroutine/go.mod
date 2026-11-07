module github.com/ggsrc/gglib/resource/goroutine

go 1.27.0

require (
	github.com/ggsrc/gglib/resource v0.1.0
	github.com/ggsrc/gglib/safego v0.1.0
	github.com/ggsrc/gglib/zerolog v0.1.0
	github.com/rs/zerolog v1.35.1
	github.com/stretchr/testify v1.12.1
)

require (
	github.com/mattn/go-colorable v0.1.16 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

// Sibling modules in this repository (scripts/replaces.sh).
replace (
	github.com/ggsrc/gglib/env => ../../env
	github.com/ggsrc/gglib/resource => ..
	github.com/ggsrc/gglib/safego => ../../safego
	github.com/ggsrc/gglib/zerolog => ../../zerolog
)
