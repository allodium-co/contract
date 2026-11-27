// Package agentpbtest provides test fakes of both ends of the agent stream,
// AgentService.StreamCommands, so the control plane and the data plane each
// test their side against the same definition of the other side.
//
// FakeControlPlane is a gRPC server for testing an agent; FakeAgent is a gRPC
// client for testing a control plane. Both run over real gRPC with the agentpb
// codec and types, so a frame that would not survive the real wire fails in a
// test too. DesiredFrame, StatusFrame, ObservedFrame and UsageFrame build
// valid frames of each payload variant from the wire contract's frozen
// goldens.
//
// Importing this package registers the agentpb codec in the test binary, as
// importing agentpb does in production.
package agentpbtest
