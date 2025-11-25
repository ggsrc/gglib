.PHONY: lint lint-fix
lint:
	@echo "--> Running linter"
	@find . -name go.mod -not -path "./go.mod" | while read modfile; do \
		dir=$$(dirname $$modfile); \
		echo "Linting $$dir"; \
		(cd "$$dir" && golangci-lint run) || exit 1; \
	done

lint-fix:
	@echo "--> Running linter auto fix"
	@find . -name go.mod -not -path "./go.mod" | while read modfile; do \
		dir=$$(dirname $$modfile); \
		echo "Linting $$dir"; \
		(cd "$$dir" && golangci-lint run --fix) || exit 1; \
	done

go-mod-tidy:
	@echo "--> Running go mod tidy"
	@find . -name go.mod -not -path "./go.mod" | while read modfile; do \
		dir=$$(dirname $$modfile); \
		echo "Tidying $$dir"; \
		(cd "$$dir" && go mod tidy) || exit 1; \
	done