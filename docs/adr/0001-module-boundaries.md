# 0001 — Core in the root module, integrations in separate modules

- Status: Accepted
- Date: 2026-09-22

## Context

The runtime must be importable without dragging in integration SDKs (Google APIs, Slack, etc.),
which are heavy and evolve at their own pace. At the same time, a single repository keeps
development simple while contracts are still changing.

## Decision

- The root module `github.com/ynxdeiv/goodeiv` holds the core and the provider adapters.
- Vendor SDKs may only be imported inside `provider/<name>`.
- Each integration is its own module at `integrations/<name>/go.mod`, importing the core.
- The core never imports `integrations/*`.
- Local development uses an unversioned `go.work` to link modules.

## Alternatives considered

- **Single module**: every core consumer would inherit all integration dependencies.
- **Separate repositories from day one**: high versioning cost while contracts are still moving.

## Consequences

- The core's `go list -deps` output is verifiable in CI.
- Integrations need their own version tags (`integrations/google/vX.Y.Z`).
- Contract changes in the core require updating integrations in the same PR or right after.
