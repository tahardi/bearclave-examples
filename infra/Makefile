# https://clarkgrubb.com/makefile-style-guide
MAKEFLAGS += --warn-undefined-variables
SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := pre-pr
.DELETE_ON_ERROR:
.SUFFIXES:

.PHONY: pre-pr
pre-pr: fmt lint sec-check-quiet

.PHONY: fmt
fmt:
	@terraform fmt -recursive

.PHONY: fmt-check
fmt-check:
	@terraform fmt -recursive -check

lint_config=$(CURDIR)/.tflint.hcl
.PHONY: lint-init
lint-init:
	@tflint --config $(lint_config) --init

.PHONY: lint
lint: lint-init
	@tflint --config $(lint_config) --recursive

.PHONY: lint-fix
lint-fix: lint-init
	@tflint --config $(lint_config) --fix --recursive

.PHONY: sec-check
sec-check:
	@trivy config \
		--config .trivy.yml \
		.

.PHONY: sec-check-quiet
sec-check-quiet:
	@trivy config \
		--config .trivy.yml \
		. > /dev/null 2>&1
