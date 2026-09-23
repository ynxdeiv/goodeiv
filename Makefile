GO ?= go
PKGS := ./...

.PHONY: setup check fmt fmt-check vet lint test race cover nocomments leakcheck tidy

setup:
	git config core.hooksPath .githooks
	chmod +x .githooks/* scripts/*.sh

check: fmt-check vet nocomments leakcheck lint race

fmt:
	gofmt -w .

fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	$(GO) vet $(PKGS)

lint:
	@command -v golangci-lint >/dev/null || { echo "golangci-lint not installed: brew install golangci-lint"; exit 1; }
	golangci-lint run

test:
	$(GO) test $(PKGS)

race:
	$(GO) test -race $(PKGS)

cover:
	$(GO) test -race -coverprofile=coverage.out $(PKGS)
	$(GO) tool cover -func=coverage.out | tail -1

nocomments:
	$(GO) run ./internal/tools/nocomments .

leakcheck:
	@./scripts/leakcheck.sh

tidy:
	$(GO) mod tidy
