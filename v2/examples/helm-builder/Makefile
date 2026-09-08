override GOLANGCI_LINT_VERSION := v2.12.2
override CONTROLLER_GEN_VERSION := v0.20.1

GOLANGCI_LINT = go run "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)"
CONTROLLER_GEN = go run "sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)"
KUBECTL ?= kubectl
CONTAINER_TOOL ?= podman
IMG ?= controller:latest

GO_MOD_VERSION := $(shell awk '/^go /{print $$2; exit}' go.mod)
export GOTOOLCHAIN := go$(GO_MOD_VERSION)

.PHONY: all
all: fmt generate vet lint test

.PHONY: fmt
fmt:
	$(GOLANGCI_LINT) fmt

.PHONY: generate
generate:
	$(CONTROLLER_GEN) object paths="./api/..."
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) rbac:roleName=helm-example paths="./internal/controller/..." output:rbac:artifacts:config=config/rbac

.PHONY: manifests
manifests: generate

.PHONY: build
build:
	mkdir -p bin
	go build -o bin/manager ./cmd

.PHONY: run
run: generate
	go run ./cmd

.PHONY: install
install: manifests
	$(KUBECTL) apply -k config/default

.PHONY: uninstall
uninstall:
	$(KUBECTL) delete -k config/default --ignore-not-found

.PHONY: deploy
deploy: build
	$(CONTAINER_TOOL) build -t $(IMG) .
	$(CONTAINER_TOOL) push $(IMG)
	$(KUBECTL) apply -k config/default
	$(KUBECTL) -n system set image deployment/helm-example-controller-manager manager=$(IMG)

.PHONY: undeploy
undeploy: uninstall

.PHONY: docker-build
docker-build: build
	$(CONTAINER_TOOL) build -t $(IMG) .

.PHONY: docker-push
docker-push:
	$(CONTAINER_TOOL) push $(IMG)

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint:
	$(GOLANGCI_LINT) run

.PHONY: test
test:
	go test -race -coverprofile=cover.out ./...

.PHONY: tidy
tidy:
	go mod tidy
