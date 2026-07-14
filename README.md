# contract

The canonical **control-plane ↔ data-plane agent wire contract** for the Allodium
lakehouse platform, versioned independently of either plane and **additive-only**.

This module is the single source of truth for the messages that cross the gRPC
stream between the (private) control plane and the (customer-run) data-plane
agent. Both planes depend on it as a released Go module; neither depends on the
other. A newer control plane must keep talking to an older agent, so the schema
here **never removes, renames, renumbers, or retypes a field** — it only adds.

```
contract ← dataplane      (agent)
contract ← controlplane   (ui)
controlplane ─X→ dataplane (wire only, no code dependency)
```

## Layout

| Path | What it is |
|------|-----------|
| [`agentpb/`](./agentpb) | The message types + codec: `ClusterCommand`, `ClusterStatus`, `AgentMessage`, and their JSON (de)serialization. Hand-written — the wire is JSON, not compiled protobuf. |
| [`clusterspec/`](./clusterspec) | Desired-state (`ClusterSpec`) and observed-status (`ClusterStatus`) schema — the single source of truth that replaced the old CRD mirrors. |
| [`agentwire/`](./agentwire) | Version-skew policy in code: `MinSupportedAgentVersion`, semver classification, and the **golden round-trip test** that fails CI on any breaking wire change. |
| [`proto/agent.proto`](./proto/agent.proto) | Human-readable **spec** of the stream (not the compiled wire format). The Go types above are canonical; this documents them. |

## Wire format

The wire is **hand-written JSON** (decision af9z.2 — no protobuf/buf codegen).
The agent dials outbound and opens one long-lived bidirectional stream; the
control plane pushes `ClusterCommand` frames, the agent replies with
`ClusterStatus`. See [`proto/agent.proto`](./proto/agent.proto) for the frame
shapes and [`agentpb/`](./agentpb) for the authoritative Go definitions.

## Versioning & compatibility

- **Additive-only.** Fields may be added; a field's name, JSON tag, or type is
  never changed or removed without the deprecation process.
- **Support window: agent N / N-1 / N-2.** The control plane supports the current
  agent minor release and the two behind it. The machine-readable floor is
  [`agentwire.MinSupportedAgentVersion`](./agentwire/version.go).
- **CI enforces backward compatibility** via the golden round-trip test
  ([`agentwire/golden_test.go`](./agentwire/golden_test.go)), wired as
  `.github/workflows/contract-compat.yml`. It fails the build on any breaking
  wire change. Full policy: [`docs/version-compatibility.md`](./docs/version-compatibility.md).

## Consuming this module

```bash
go get github.com/allodium-co/contract@v0.1.0
```

Tags are **forward-only**: a published tag is never moved. A fix ships as the
next patch (`v0.1.1`), always additively.

## License

Apache 2.0 — see [`LICENSE`](./LICENSE) and [`NOTICE`](./NOTICE). Security policy:
[`SECURITY.md`](./SECURITY.md). Contributions: [`CONTRIBUTING.md`](./CONTRIBUTING.md).
