-include local.mk

override GOLANGCI_LINT_VERSION := v2.12.2
override CONTROLLER_GEN_VERSION := v0.20.1

GOLANGCI_LINT = go run "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)"
LINT_FLAGS ?=
CONTROLLER_GEN = go run "sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)"
CONTAINER_TOOL ?= podman
KUBECTL ?= kubectl
IMG ?= ttl.sh/odh-platform-utilities-v2-example:24h
MODULES_DIR ?= config/modules
MODULE_NAMESPACE ?= default

GO_MOD_VERSION := $(shell awk '/^go /{print $$2; exit}' go.mod)
export GOTOOLCHAIN := go$(GO_MOD_VERSION)

.PHONY: generate manifests
generate:
	$(CONTROLLER_GEN) object paths="./api/..."

manifests: generate
	$(CONTROLLER_GEN) crd paths="./api/platform/..." output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) crd paths="./api/kserve/..." output:crd:artifacts:config=config/modules/kserve/charts/charts/module/templates
	mv config/modules/kserve/charts/charts/module/templates/kserve.example.odh.io_kserves.yaml config/modules/kserve/charts/charts/module/templates/crd.yaml
	$(CONTROLLER_GEN) crd paths="./api/aigateway/..." output:crd:artifacts:config=config/modules/aigateway/charts/charts/module/templates
	mv config/modules/aigateway/charts/charts/module/templates/aigateway.example.odh.io_aigateways.yaml config/modules/aigateway/charts/charts/module/templates/crd.yaml

.PHONY: fmt vet lint test test-e2e test-e2e-platform test-e2e-serving test-e2e-compile
fmt:
	$(GOLANGCI_LINT) fmt

vet:
	go vet ./...

lint:
	$(GOLANGCI_LINT) run $(LINT_FLAGS) --build-tags=e2e

test:
	go test -race ./api/... ./pkg/... ./internal/...

test-e2e-compile:
	go test -tags=e2e -run '^$$' ./test/e2e

test-e2e:
	@set -e; if ! $(CONTAINER_TOOL) info >/dev/null 2>&1; then echo "SKIP: container runtime is unavailable"; exit 0; fi; $(MAKE) image; KIND_PROVIDER=$(CONTAINER_TOOL) IMG=$(IMG) go test -v -race -count=1 -parallel=1 -tags=e2e -timeout=15m ./test/e2e

test-e2e-platform:
	@set -e; if ! $(CONTAINER_TOOL) info >/dev/null 2>&1; then echo "SKIP: container runtime is unavailable"; exit 0; fi; KIND_PROVIDER=$(CONTAINER_TOOL) IMG=$(IMG) go test -v -race -count=1 -tags=e2e -run '^TestPlatformAndModuleOnKind$$' -timeout=15m ./test/e2e

test-e2e-serving:
	@set -e; if ! $(CONTAINER_TOOL) info >/dev/null 2>&1; then echo "SKIP: container runtime is unavailable"; exit 0; fi; KIND_PROVIDER=$(CONTAINER_TOOL) IMG=$(IMG) go test -v -race -count=1 -tags=e2e -run '^TestPlatformAndServingOnKind$$' -timeout=15m ./test/e2e

.PHONY: build image install run/platform run/serving
build:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -o bin/manager ./cmd

image: build
	$(CONTAINER_TOOL) build -f Containerfile -t $(IMG) .
	$(CONTAINER_TOOL) push $(IMG)

install: manifests
	$(KUBECTL) apply -f config/crd/bases/example.platform.odh.io_platforms.yaml
	$(KUBECTL) apply -f config/crd/bases/example.platform.odh.io_platformmodules.yaml
	$(KUBECTL) apply -f config/crd/bases/example.platform.odh.io_servings.yaml

run/platform:
	PLATFORM_MODULES_DIR=$(MODULES_DIR) PLATFORM_MODULE_IMAGE=$(IMG) PLATFORM_MODULE_NAMESPACE=$(MODULE_NAMESPACE) go run ./cmd run controller platform

run/serving:
	PLATFORM_MODULES_DIR=$(MODULES_DIR) go run ./cmd run controller serving

.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage: make <target>\n\n"} /^[a-zA-Z_0-9\/-]+:.*?##/ {printf "%-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
