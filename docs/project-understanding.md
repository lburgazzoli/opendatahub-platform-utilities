# ODH Platform Utilities — Project Understanding

The repository contains the retained v1 root module and the consolidated v2
module. New controller development targets v2; the root module remains for
existing v1 consumers until a separate compatibility decision removes it.

## Modules

| Module | Directory | Role |
| --- | --- | --- |
| `github.com/opendatahub-io/odh-platform-utilities` | `/` | Retained v1 platform and Kubernetes utilities |
| `github.com/opendatahub-io/odh-platform-utilities/v2` | `v2/` | Current platform contract, actions, pipeline, reconciler, and Kubernetes helpers |
| `github.com/opendatahub-io/odh-platform-utilities/testkit/kind` | `testkit/kind/` | Isolated Kind Go-library test engine |
| `github.com/opendatahub-io/odh-platform-utilities/testkit/integration` | `testkit/integration/` | Live-cluster integration-test harness |
| `github.com/opendatahub-io/odh-platform-utilities/flakiness` | `flakiness/` | CI artifact, runtime, quarantine, and Jira tooling |

The v2 module is developed through `v2/go.work` and the testkit modules are
developed through `testkit/go.work`. Local module composition uses workspaces,
not `replace` directives.

## V2 architecture

V2 uses one implementation of each retained platform capability:

- `api` contains the platform wire contract and accessor interfaces.
- `pkg/platform` contains conditions, metadata, release, and validation logic.
- `pkg/kube` contains Kubernetes resource, ownership, singleton, and admission
  helpers.
- `pkg/action` contains programmatic actions and their thin pipeline adapters.
- `pkg/controller/pipeline` owns request extensions and phased execution.
- `pkg/controller/reconciler` owns controller-runtime lifecycle, finalizers,
  status persistence, and optional dynamic ownership watches.

The normal action flow is:

```text
controller-owned rendering
  -> before actions
  -> main actions such as deploy
  -> after actions such as GC and dynamic ownership
  -> status persistence
```

Deletion is a separate cleanup lifecycle. Cleanup actions run only after the
primary object has a deletion timestamp, and the reconciler removes its
finalizer only after cleanup completes or the configured cleanup deadline is
reached.

Actions expose a programmatic `Run` method and a thin `Execute` adapter. Their
results are quantitative where useful so the pipeline and future metrics can
consume counts without collecting object lists unnecessarily.

## Integration testing

The isolated Kind engine uses the Kind Go provider API and never invokes the
Kind binary. V2's integration suite lives in `v2/test/integration` and links
the engine through `v2/go.work`. The reusable live-cluster harness remains in
`testkit/integration`; it is not a v2 runtime dependency.

Run v2 checks with:

```bash
make -C v2 test
make -C v2 test-integration
```

The integration suite explicitly skips when Docker or Podman is unavailable.

## Legacy compatibility

The root module is intentionally not imported by v2 packages. Its v1 packages
remain available for existing consumers and are not compatibility aliases for
v2 types. New code must not add v1 imports to v2 or recreate removed wrapper
packages.
