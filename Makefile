# SHFMT = $(shell which shfmt 2>/dev/null)
# GOIMPORTS := $(shell which goimports 2>/dev/null)

help: ## Display this help screen
	@printf "Help doc:\nUsage: make [command]\n"
	@printf "[command]\n"
	@grep -h -E '^([a-zA-Z_-]|\%)+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: init

init:
	@if ! command -v shfmt &> /dev/null; then \
		echo "shfmt not found, installing..."; \
		GO111MODULE=on go install mvdan.cc/sh/v3/cmd/shfmt@v3.7.0; \
	fi
	@if ! command -v goimports &> /dev/null; then \
		echo "goimports not found, installing..."; \
		GO111MODULE=on go install golang.org/x/tools/cmd/goimports@v0.24.0; \
	fi
	@if ! command -v shellcheck &> /dev/null; then \
		echo "shellcheck not found, installing..."; \
		apt install shellcheck; \
	fi
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "golangci-lint not found, installing..."; \
		GO111MODULE=on go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2; \
	fi

.PHONY: test bench

test: ## Run unittests
	@go clean -testcache
	@go test -short -race `go list ./...`

bench: ## Run benchmark of all
	@go test ./... -v -bench=.

.PHONY: fmt_proto fmt_shell fmt_go

fmt: init fmt_shell fmt_go ## file format

fmt_shell: ## format .sh files
	@find . -name '*.sh' -not -path "./vendor/*" | xargs shfmt -w -s -i 4 -ci -bn

fmt_go: ## format .go files
	@find . -name '*.go' -not -path "./vendor/*" | xargs gofmt -s -w
	@find . -name '*.go' -not -path "./vendor/*" | xargs goimports -l -w -local "github.com/danielhookx/eventbus"

.PHONY: checkgofmt linter linter_test

check: init checkgofmt linter ## check format and linter

checkgofmt: ## get all go files and run go fmt on them
	@files=$$(find . -name '*.go' -not -path "./vendor/*" | xargs gofmt -l -s); if [ -n "$$files" ]; then \
		  echo "Error: 'make fmt' needs to be run on:"; \
		  find . -name '*.go' -not -path "./vendor/*" | xargs gofmt -l -s ;\
		  exit 1; \
		  fi;
	@files=$$(find . -name '*.go' -not -path "./vendor/*" | xargs goimports -l -w); if [ -n "$$files" ]; then \
		  echo "Error: 'make fmt' needs to be run on:"; \
		  find . -name '*.go' -not -path "./vendor/*" | xargs goimports -l -w ;\
		  exit 1; \
		  fi;

linter: ## Use gometalinter check code, ignore some unserious warning
	@./golinter.sh "filter"
	@find . -name '*.sh' -not -path "./vendor/*" | xargs shellcheck

linter_test: ## Use gometalinter check code, for local test
	@./golinter.sh "test" "${p}"
	@find . -name '*.sh' -not -path "./vendor/*" | xargs shellcheck
