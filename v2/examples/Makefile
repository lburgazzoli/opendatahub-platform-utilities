.PHONY: all
all: fmt generate vet lint test

.PHONY: fmt
fmt:
	$(MAKE) -C helm-builder fmt

.PHONY: generate
generate:
	$(MAKE) -C helm-builder generate

.PHONY: vet
vet:
	$(MAKE) -C helm-builder vet

.PHONY: lint
lint:
	$(MAKE) -C helm-builder lint

.PHONY: test
test:
	$(MAKE) -C helm-builder test

.PHONY: build
build:
	$(MAKE) -C helm-builder build

.PHONY: install
install:
	$(MAKE) -C helm-builder install

.PHONY: uninstall
uninstall:
	$(MAKE) -C helm-builder uninstall

.PHONY: deploy
deploy:
	$(MAKE) -C helm-builder deploy

.PHONY: undeploy
undeploy:
	$(MAKE) -C helm-builder undeploy

.PHONY: tidy
tidy:
	$(MAKE) -C helm-builder tidy
