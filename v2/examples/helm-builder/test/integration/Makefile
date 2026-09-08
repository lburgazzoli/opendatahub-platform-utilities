GOLANGCI_LINT_VERSION := v2.12.2

GOLANGCI_LINT = go run "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)"

GO_MOD_VERSION := $(shell awk '/^go /{print $$2; exit}' go.mod)
export GOTOOLCHAIN := go$(GO_MOD_VERSION)

.PHONY: test
test:
	go test -race -count=1 -tags=integration .

.PHONY: fmt
fmt:
	$(GOLANGCI_LINT) fmt

.PHONY: lint
lint:
	$(GOLANGCI_LINT) run --build-tags=integration

.PHONY: vet
vet:
	go vet -tags=integration .

.PHONY: tidy
tidy:
	go mod tidy
