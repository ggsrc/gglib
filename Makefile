# Every module is checked on its own, with GOWORK=off, the same way a
# consumer resolves it. For cross-module development run `make work` to create
# a local go.work; go.work and go.work.sum are never committed.
MODULES := $(shell find . -name go.mod -not -path './.git/*' -exec dirname {} \; | sed 's|^\./||' | sort)
export GOWORK := off

.PHONY: modules tidy tidy-check build vet test lint lint-fix work go-mod-tidy go-mod-update

modules:
	@printf '%s\n' $(MODULES)

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

lint:
	@for m in $(MODULES); do echo "--> lint $$m"; (cd $$m && golangci-lint run) || exit 1; done

# Fixes what it can in every module, then fails if any module still has issues.
lint-fix:
	@status=0; for m in $(MODULES); do echo "--> lint-fix $$m"; (cd $$m && golangci-lint run --fix) || status=1; done; exit $$status

# Local workspace over all modules. Not committed (see .gitignore).
work:
	@rm -f go.work go.work.sum
	@GOWORK= go work init $(addprefix ./,$(MODULES))
	@echo "--> wrote go.work with $(words $(MODULES)) modules"

# Kept for existing muscle memory.
go-mod-tidy: tidy

go-mod-update:
	@for m in $(MODULES); do echo "--> update $$m"; (cd $$m && go get -u ./... && go mod tidy) || exit 1; done
