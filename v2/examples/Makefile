.PHONY: all
all: fmt generate vet lint test

.PHONY: fmt
fmt:
	$(MAKE) -C helm-builder fmt
	$(MAKE) -C helm-plain fmt

.PHONY: generate
generate:
	$(MAKE) -C helm-builder generate
	$(MAKE) -C helm-plain generate

.PHONY: vet
vet:
	$(MAKE) -C helm-builder vet
	$(MAKE) -C helm-plain vet

.PHONY: lint
lint:
	$(MAKE) -C helm-builder lint
	$(MAKE) -C helm-plain lint

.PHONY: test
test:
	$(MAKE) -C helm-builder test
	$(MAKE) -C helm-plain test

.PHONY: build
build:
	$(MAKE) -C helm-builder build
	$(MAKE) -C helm-plain build

.PHONY: install
install:
	$(MAKE) -C helm-builder install
	$(MAKE) -C helm-plain install

.PHONY: uninstall
uninstall:
	$(MAKE) -C helm-builder uninstall
	$(MAKE) -C helm-plain uninstall

.PHONY: deploy
deploy:
	$(MAKE) -C helm-builder deploy
	$(MAKE) -C helm-plain deploy

.PHONY: undeploy
undeploy:
	$(MAKE) -C helm-builder undeploy
	$(MAKE) -C helm-plain undeploy

.PHONY: tidy
tidy:
	$(MAKE) -C helm-builder tidy
	$(MAKE) -C helm-plain tidy
