# Helm builder controller

This example is a controller-runtime controller that renders a Helm chart and
deploys its resources through the v2 reconciler builder. Each `HelmComponent`
uses the configured module namespace, and the example chart renders a
ConfigMap there.

## Configuration and startup

`make run` loads startup configuration from environment variables. Set:

- `HELM_EXAMPLE_CHART` — required path to the Helm chart.
- `HELM_EXAMPLE_NAMESPACE` — required namespace for the module and rendered
  resources.
- `HELM_EXAMPLE_HEALTH_PROBE_BIND_ADDRESS` — optional health probe address;
  defaults to `:8081`.

Set `HELM_EXAMPLE_CONFIGURATION_PATH` to an optional directory containing the
configuration files `chart-path`, `namespace`, and
`controller.health.bind-address`. Environment variables override file values.

For a local run, for example:

```sh
HELM_EXAMPLE_CHART=./config/chart \
HELM_EXAMPLE_NAMESPACE=default \
make run
```

The deployment manifest copies the chart to `/opt/charts` and configures the
`default` namespace. Apply the generated CRD and a `HelmComponent` resource
with `make install` before exercising the controller against a cluster.

## Make targets

From this directory:

- `make all` runs formatting, generation, vet, lint, unit tests, and the
  Kind-backed integration test.
- `make fmt`, `make generate`, `make vet`, and `make lint` run the corresponding
  checks.
- `make test` runs unit tests and does not require a container runtime.
- `make test-integration` runs the in-module Kind integration test.
- `make build` and `make run` build or run the controller.
- `make install` and `make uninstall` apply or remove the Kubernetes manifests.
- `make docker-build IMG=...`, `make docker-push IMG=...`, `make deploy IMG=...`,
  and `make undeploy` manage the controller image and deployment.
- `make tidy` updates the example module's dependencies.

From `v2/examples`, the aggregate `all` and `test-integration` targets provide
the same validation entry points for the example.
