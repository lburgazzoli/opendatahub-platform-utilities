# Platform modules example

This example shows how to compose controllers with the v2 framework. A
`Serving` resource selects Kserve and AI Gateway. The Serving controller writes
the selected modules into the singleton `Platform` resource. The Platform
controller creates a `PlatformModule` for each selection. Each
`PlatformModule` installs its module CRD and a controller Deployment from a
chart. The module controllers reconcile typed CRs and report readiness through
the shared v2 status helpers.

The controllers live in `internal/controller/serving`, `platform`, and
`module`. Each uses `reconciler.For`. Serving uses dynamic GVK watches because
the module CRDs are installed after its manager starts.

## Module bundles

`config/modules/kserve` and `config/modules/aigateway` each contain a
`module.yaml`. It names the CRD, GVK, chart, and chart values.
`pkg/modules.Load` reads the definitions at startup. The shared controller
chart derives the CRD name, group, version, kind, and resource names from each
definition, then installs it with the module controller. The module controller
uses the same definition to choose its typed
prototype. The API packages are separate groups under `api/kserve` and
`api/aigateway`.

`make manifests` regenerates the API code and reference CRDs. The module chart
contains the CRD template used at runtime.

## Run locally

Install only the Platform, PlatformModule, and Serving CRDs first:

```sh
make install
```

Build and publish the controller image used by module Deployments. The default
image is `ttl.sh/odh-platform-utilities-v2-example:24h`; set `IMG` to use a
different registry or tag.

```sh
make image
```

In two terminals, run the controllers against the current kubeconfig:

```sh
make run/platform
make run/serving
```

Create a Serving instance:

```sh
kubectl apply -f config/samples/serving.yaml
```

The installed image runs `manager run module kserve` or
`manager run module aigateway` for each selected module. To run one of these
module controllers locally, execute `go run ./cmd run module kserve` or
`go run ./cmd run module aigateway` from this directory after its CRD exists.
The Platform and Serving commands are `go run ./cmd run controller platform`
and `go run ./cmd run controller serving`.

Add `example.platform.odh.io/simulated-failure` with a nonempty value to a
Kserve or AI Gateway CR to make its controller return an error. Its
`SimulationActive` condition records the annotation value. The v2 reconciler sets
`ProvisioningSucceeded=False` and `Ready=False`; removing the annotation
restores them. `ModuleConfigured` and `SimulationActive` are example conditions
that do not contribute to readiness.

## Tests

`make test` and `make lint` check the example locally. `make test-e2e` uses
the Kind Go library to create disposable clusters and tests the module and
Serving flows. It builds and publishes the image first. The test skips when
the configured container runtime is unavailable. Use `CONTAINER_TOOL=podman`
or `CONTAINER_TOOL=docker` to select a runtime.
