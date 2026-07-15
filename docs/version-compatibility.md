# Agent ↔ Control-Plane Version Compatibility Policy

`controlplane` and `dataplane` ship on **independent cadences**. A customer
runs the data-plane agent inside their own environment and upgrades it on
their own schedule, while the control plane is a hosted service that upgrades
continuously. The gRPC stream between them therefore carries **version
skew**: a current control plane must keep talking to an older agent, and
(briefly, during a staged rollout) an older control plane must tolerate a
newer agent. This document is that policy.

## TL;DR

- **Wire is JSON, hand-written** — no protobuf/buf. The schema lives in this
  shared Go contract module.
- **The schema is additive-only.** Add fields; never remove, rename, renumber, or
  retype one. A breaking change requires the deprecation process below.
- **Support window: agent N, N-1, N-2.** The control plane supports the current
  agent minor release and the two behind it. The machine-readable floor is
  [`agentwire.MinSupportedAgentVersion`](../agentwire/version.go).
- **CI enforces backward-compat** with a golden round-trip test
  ([`agentwire/golden_test.go`](../agentwire/golden_test.go)),
  wired as [`.github/workflows/contract-compat.yml`](../.github/workflows/contract-compat.yml).
  It fails the build on any breaking wire change.

## What the agent reports

The agent already advertises its release version two ways, so the control plane
never has to guess:

1. **gRPC metadata `agent-version`** at stream connect (set by the data-plane
   agent's connector) — read at handshake and persisted to the
   `agent_connections` row.
2. **`ClusterStatus.AgentVersion`** in the observed-status frames
   ([`clusterspec/status.go`](../clusterspec/status.go)).

The value is the data-plane **release semver** — the agent image tag / chart
`appVersion`, stamped by the data plane's release pipeline from the `v*.*.*`
release tag (`AGENT_VERSION` env in the agent Deployment). Dev builds report a
non-semver tag such as `latest`; the control plane classifies those as
`SupportUnknown` and admits them without a skew claim.

## The support window (N / N-1 / N-2)

The control plane supports the current agent release and the **two minor
releases** behind it:

| Reported agent version                | Classification    | Control-plane behaviour |
|----------------------------------------|-------------------|-------------------------|
| ≥ `MinSupportedAgentVersion`           | `SupportOK`       | Normal — inside the window. |
| Newer than the control plane           | `SupportOK`       | Accepted — additive-only schema makes a newer agent safe for an older control plane; unknown fields are ignored. |
| < `MinSupportedAgentVersion`           | `SupportTooOld`   | Admitted for MVP, but a `WARN` is logged at handshake so an operator sees the skew before it becomes a support case. |
| Unparseable (`latest`, git SHA, empty) | `SupportUnknown`  | Admitted; dev / BYO build — no skew guarantee. |

The window is expressed in code as two constants in the shared contract module
so control plane and agent cannot disagree:

- `MinSupportedAgentVersion` — the enforced floor (currently `0.1.0`).
- `SupportWindowMinors` — the descriptive width of the window (`2` = N-2).

Classification is a pure function, `agentwire.ClassifyAgentVersion(reported)`,
called at the control plane's agentserver handshake (private repo).

### Why `SupportTooOld` warns instead of rejecting (MVP)

Hard-rejecting an old agent would take down a customer's data plane the moment
the control plane rolls a release forward — the exact failure the support window
exists to *prevent*. For MVP the control plane therefore **admits** an
out-of-window agent and flags it (log/telemetry) so the customer can be nudged
to upgrade. Flipping `SupportTooOld` to a hard connection reject is a deliberate
future step, gated on: (a) a customer-facing upgrade-notice path, and (b)
confidence that no supported customer is pinned below the floor. Track it as a
follow-up before GA.

### Raising the floor

As a release ages out of the N-2 window, release engineering bumps
`MinSupportedAgentVersion` to the new floor in the same change that documents the
deprecation. The floor only ever moves **forward**. Because it is a compile-time
constant in the shared module, the bump is reviewed like any code change and is
covered by `TestMinSupportedIsParseable`.

## Additive-only schema discipline

The JSON message schema is the two type groups in this module:

- **`agentpb`** — the stream envelope and frames: `AgentMessage`, `ClusterStatus`
  (heartbeat), `UsageReport`, `WorkloadUsage`, `ResourceUsage`.
- **`clusterspec`** — desired state and observed status: `ClusterSpec`,
  `WarehouseSpec`, `SparkClusterSpec`, `ClusterStatus`, `WarehouseStatus`,
  `SparkClusterStatus`.

The spec doc [`proto/agent.proto`](../proto/agent.proto) is a
human-readable mirror of the same contract and follows the same rules.

**Allowed (additive, no version negotiation needed):**

- Add a new optional field (`,omitempty`) to any message.
- Add a new message type or a new `AgentMessage` payload variant.
- Add a new enum-like string value where the reader already tolerates unknowns
  (e.g. a new `ResourceType`, a new `worker_mode` — validated as an open set).

**Forbidden (breaking — requires the deprecation process):**

- Removing a field, or changing its JSON tag (a rename on the wire).
- Changing a field's type (`int` → `string`, scalar → object, etc.).
- Changing the meaning/units of an existing field.
- Tightening a reader to reject a value it used to accept.
- Making a previously optional field required.

Every additive change **increments `agentwire.SchemaVersion` by one**. That
integer is published alongside the neutral JSON Schema so a
bring-your-own agent can tell which generation it targets.

## The CI backward-compatibility gate

There is no `buf breaking`; the equivalent gate is a **golden
round-trip test**. `agentwire/testdata/agentmessage_*.json`
are **frozen** samples of what an *older* peer emits, one per `AgentMessage`
payload variant, with every field of that generation populated. For each golden
the test:

1. decodes it into the **current** Go types,
2. re-encodes, and
3. asserts every key/value in the golden **survived** the round trip (a superset
   check).

That direction is exactly additive-only discipline:

- **Add a field** → the current types emit an extra key the golden lacks; the
  superset check ignores it → **PASS**.
- **Remove / rename / retype a field** → the golden's key/value no longer
  round-trips → **FAIL**, with the offending JSON path named.

### Rules for contributors

- **Never edit an existing golden to make a red test green.** That erases the
  historical baseline the gate depends on. A red gate means you made a breaking
  change — revert to additive, or run the deprecation process.
- **To land an additive change:** leave the existing goldens untouched (they
  pass). Optionally add a *new* golden that freezes the new field as a future
  baseline.

Run it locally with `go test ./...` from the repo root. In CI it is
[`contract-compat.yml`](../.github/workflows/contract-compat.yml), which runs on
every push and PR.

## Deprecation process (the only way to make a breaking change)

Removing or reshaping a wire field is a multi-release migration, never a single
edit:

1. **Add the replacement additively.** Introduce the new field/type/value
   alongside the old one; bump `SchemaVersion`. Both planes now populate and read
   both. Ship it.
2. **Migrate readers.** In a later release, switch the control plane and the
   agent to prefer the new field, falling back to the old one when only an older
   peer populated it.
3. **Wait out the window.** Keep the old field on the wire until every release
   that still emits it has aged past `MinSupportedAgentVersion` (N-2). Raise the
   floor as part of this step.
4. **Remove the old field.** Only now delete it and add a *new* golden reflecting
   the removal. This is the one change that legitimately turns the gate red
   against the *old* golden — so it is done deliberately, retiring that golden in
   the same commit, with this document cited in the PR.

A breaking change that skips steps 1–3 is the failure mode this policy exists to
prevent; the CI gate is what makes skipping it impossible by accident.

## Ownership

This document, the contract module, and the compatibility gate live in the
standalone `contract` repo, with no control-plane or data-plane coupling.
