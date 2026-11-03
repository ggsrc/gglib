# Every module is checked on its own with GOWORK=off. Sibling gglib modules
# resolve to the directories in this repository through replace directives
# (scripts/replaces.sh), so a change that spans modules is checked as a whole.
MODULES := $(shell find . -name go.mod -not -path './.git/*' -exec dirname {} \; | sed 's|^\./||' | sort)
export GOWORK := off

.PHONY: modules replaces tidy tidy-check build vet test cover lint lint-fix vulncheck release-prepare go-mod-tidy go-mod-update

modules:
	@printf '%s\n' $(MODULES)

# Adds a replace directive for every sibling module a go.mod needs.
replaces:
	@./scripts/replaces.sh

tidy:
	@for m in $(MODULES); do echo "--> tidy $$m"; (cd $$m && go mod tidy) || exit 1; done

tidy-check:
	@for m in $(MODULES); do echo "--> tidy-check $$m"; (cd $$m && go mod tidy -diff) || exit 1; done

build:
	@for m in $(MODULES); do echo "--> build $$m"; (cd $$m && go build -mod=readonly ./...) || exit 1; done

vet:
	@for m in $(MODULES); do echo "--> vet $$m"; (cd $$m && go vet -mod=readonly ./...) || exit 1; done

test:
	@for m in $(MODULES); do echo "--> test $$m"; (cd $$m && go test -mod=readonly -race -count=1 ./...) || exit 1; done

# Tests with coverage, checked against the floors in .github/coverage.txt.
cover:
	@for m in $(MODULES); do echo "--> cover $$m"; (cd $$m && go test -mod=readonly -race -count=1 -covermode=atomic -coverprofile=coverage.out ./... >/dev/null && $(CURDIR)/scripts/coverage-check.sh $$m coverage.out; status=$$?; rm -f coverage.out; exit $$status) || exit 1; done

lint:
	@for m in $(MODULES); do echo "--> lint $$m"; (cd $$m && golangci-lint run) || exit 1; done

# Fixes what it can in every module, then fails if any module still has issues.
lint-fix:
	@status=0; for m in $(MODULES); do echo "--> lint-fix $$m"; (cd $$m && golangci-lint run --fix) || status=1; done; exit $$status

vulncheck:
	@for m in $(MODULES); do echo "--> vulncheck $$m"; (cd $$m && govulncheck ./...) || exit 1; done

# Kept for existing muscle memory.
go-mod-tidy: tidy

go-mod-update:
	@for m in $(MODULES); do echo "--> update $$m"; (cd $$m && go get -u ./... && go mod tidy) || exit 1; done
