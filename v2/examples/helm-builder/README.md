# Helm builder controller

This example is a complete controller-runtime project that renders a Helm
chart and deploys its resources with the v2 action pipeline.

It uses the v2 manager wrapper and cache-coherent client. Its Helm render
action is registered through `reconciler.Builder`; the plain example uses the
same API, chart, deploy options, status conditions, and manager wrapper while
calling `Reconcile` and `deploy.Action.Run` directly.

The controller is registered from `internal/controller` with:

```go
reconciler.For(manager, v1alpha1.NewHelmComponent()).
    WithActionFunc(reconcilerValue.render).
    WithAction(deploy.New(deploy.WithFieldOwner("helm-example"))).
    Build()
```

The `-chart` argument must point to a Helm chart whose templates use
`.Values.name`. Apply the CRD in `config/crd/bases` and the sample in
`config/samples` before running the example. `make run` uses the embedded
example chart; `make docker-build IMG=...` builds an image containing the same
chart, and `make deploy IMG=...` installs the generated controller manifests.
Use the workspace in `examples/go.work` when building either module.
