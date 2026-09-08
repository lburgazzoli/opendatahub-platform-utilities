# Plain Helm controller

This example is a complete controller-runtime project that performs the same
Helm deployment as the builder example, but wires the lifecycle explicitly.

It uses the same v2 manager wrapper, Helm chart, deploy options, status
conditions, and API as the builder example. The only difference is that it
does not use `reconciler.Builder` or the action pipeline.

`Reconcile` loads the primary object, renders the Helm chart, and invokes the
deploy action directly with `deploy.Action.Run`. It does not use the v2
reconciler, pipeline, or reconciler builder.

The `-chart` argument must point to a Helm chart whose templates use
`.Values.name`. Apply the CRD in `config/crd/bases` and the sample in
`config/samples` before running the example. `make run` uses the embedded
example chart; `make docker-build IMG=...` builds an image containing the same
chart, and `make deploy IMG=...` installs the generated controller manifests.
Use the workspace in `examples/go.work` when building either module.
