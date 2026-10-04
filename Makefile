# https://clarkgrubb.com/makefile-style-guide
MAKEFLAGS += --warn-undefined-variables
SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := pre-pr
.DELETE_ON_ERROR:
.SUFFIXES:

.PHONY: pre-pr
pre-pr: tidy lint fix test-unit test-examples pluckmd tf

.PHONY: fix
fix:
	@go fix ./...

# https://golangci-lint.run/welcome/install/#install-from-sources
# They do not recommend using golangci-lint via go tool directive
# as there are still bugs, but I want to try out go tool and work
# uses an old version of golangci-lint. So, I don't mind guinea
# pigging go tool and using a new version of golangci-lint in here
lint_modfile=modfiles/golangci-lint/go.mod
.PHONY: lint
lint:
	@go tool -modfile=$(lint_modfile) golangci-lint run --config .golangci.yaml

.PHONY: lint-fix
lint-fix:
	@go tool -modfile=$(lint_modfile) golangci-lint run --config .golangci.yaml --fix

.PHONY: pluckmd
pluckmd:
	@pluckmd --dir .

.PHONY: tidy
tidy:
	@go mod tidy

.PHONY: tf
tf:
	@make -C ./infra/

.PHONY: test-unit
test-unit: tidy test-internal-unit test-nonclave

.PHONY: test-internal-unit
test-internal-unit:
	@go test -v -count=1 -race ./internal/...

.PHONY: test-nonclave
test-nonclave:
	@go test -v -count=1 -race \
		./hello-world/nonclave/ \
		./hello-http/nonclave/ \
		./hello-https/nonclave/ \
		./hello-expr/nonclave/ \
		./hello-cel/nonclave/ \
		./hello-iac/nonclave/

.PHONY: test-examples
test-examples: \
	hello-world \
	hello-http \
	hello-https \
	hello-expr \
	hello-cel \
	hello-iac

.PHONY: hello-world
hello-world:
	@make -C ./hello-world/

.PHONY: hello-http
hello-http:
	@make -C ./hello-http/

.PHONY: hello-https
hello-https:
	@make -C ./hello-https/

.PHONY: hello-expr
hello-expr:
	@make -C ./hello-expr/

.PHONY: hello-cel
hello-cel:
	@make -C ./hello-cel/

.PHONY: hello-iac
hello-iac:
	@make -C ./hello-iac/

.PHONY: clean
clean:
	@make -C ./hello-world/ clean
	@make -C ./hello-http/ clean
	@make -C ./hello-https/ clean
	@make -C ./hello-expr/ clean
	@make -C ./hello-cel/ clean
	@make -C ./hello-iac/ clean
