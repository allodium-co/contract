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
| [`agentpbtest/`](./agentpbtest) | Test fakes of both ends of the stream, `FakeControlPlane` and `FakeAgent`, plus builders for a valid frame of each payload variant. For the planes' tests only. |
| [`agentwire/`](./agentwire) | Version-skew policy in code: `MinSupportedAgentVersion`, semver classification, and the **golden round-trip test** that fails CI on any breaking wire change. |
| [`proto/agent.proto`](./proto/agent.proto) | Human-readable **spec** of the stream (not the compiled wire format). The Go types above are canonical; this documents them. |

## Wire format

The wire is **hand-written JSON** — no protobuf/buf codegen.
The agent dials outbound and opens one long-lived bidirectional stream; the
control plane pushes `ClusterCommand` frames, the agent replies with
`ClusterStatus`. See [`proto/agent.proto`](./proto/agent.proto) for the frame
shapes and [`agentpb/`](./agentpb) for the authoritative Go definitions.

## Testing a plane against the other end

[`agentpbtest`](./agentpbtest) lets each plane test its side of the stream
without deploying the other. Both fakes run real gRPC with the `agentpb` codec
and types, so a frame that would not survive the wire fails in a test too.

| Fake | Use it to test | What it does |
|------|----------------|--------------|
| `FakeControlPlane` | the agent (dataplane) | Listens on a loopback port. `Accept` returns each stream the agent opens, with its metadata. `WaitFor` and `Received` read the frames the agent sent. `SendDesired` and `Send` push frames down. `Close(err)` ends a stream with a gRPC status, to simulate a disconnect. |
| `FakeAgent` | the control plane (controlplane) | Dials an address and opens one stream, with `WithMetadata` for the agent's identity. `SendStatus` (heartbeat), `SendObserved` and `SendUsage` send frames up. `WaitForDesired`, `WaitFor` and `Received` read what came down. `Err` reports how the control plane ended the stream. |

`DesiredFrame`, `StatusFrame`, `ObservedFrame` and `UsageFrame` return a fresh,
valid frame of each payload variant, equal to the newest frozen golden in
`agentwire/testdata`. Change fields on the returned value to shape the case a
test needs. `HasDesired`, `HasStatus`, `HasObserved` and `HasUsage` are
matchers for `WaitFor`.

`WaitFor` returns the first matching frame received so far, or waits for one;
it does not consume frames, so calling it twice returns the same frame.
`WaitForNext` only matches frames received after the call: use it to wait for
the effect of something the test just did. A frame the codec cannot encode
makes `Send` fail and ends the stream, as on the real wire.

An agent's test points the agent at a `FakeControlPlane`:

```go
cp := agentpbtest.NewFakeControlPlane(t)
go runAgentUnderTest(ctx, cp.Addr()) // dials plaintext

stream, err := cp.Accept(ctx)
// ...
if err := stream.SendDesired(agentpbtest.DesiredFrame().Desired); err != nil {
	t.Fatal(err)
}
// The agent also reports observed state on connect; WaitForNext skips frames
// that arrived before this call.
observed, err := stream.WaitForNext(ctx, agentpbtest.HasObserved)
```

A control plane's test points a `FakeAgent` at its gRPC server:

```go
agent := agentpbtest.NewFakeAgent(t, lis.Addr().String(),
	agentpbtest.WithMetadata("agent-id", "agent-1", "workspace-id", "ws-1"))

if err := agent.SendUsage(agentpbtest.UsageFrame().Usage); err != nil {
	t.Fatal(err)
}
desired, err := agent.WaitForDesired(ctx)
```

Every wait takes a context, so bound it with a timeout. Both fakes shut down
when the test ends.

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
