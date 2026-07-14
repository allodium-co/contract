package agentpb

import (
	"context"

	"google.golang.org/grpc"
)

const streamCommandsMethod = "/agent.v1.AgentService/StreamCommands"

// AgentServiceServer is implemented by the control-plane gRPC server.
type AgentServiceServer interface {
	StreamCommands(AgentService_StreamCommandsServer) error
}

// AgentService_StreamCommandsServer is the server-side stream handle.
type AgentService_StreamCommandsServer interface {
	Send(*AgentMessage) error
	Recv() (*AgentMessage, error)
	grpc.ServerStream
}

type agentServiceStreamCommandsServer struct{ grpc.ServerStream }

func (x *agentServiceStreamCommandsServer) Send(m *AgentMessage) error {
	return x.ServerStream.SendMsg(m)
}
func (x *agentServiceStreamCommandsServer) Recv() (*AgentMessage, error) {
	m := new(AgentMessage)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _AgentService_StreamCommands_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(AgentServiceServer).StreamCommands(&agentServiceStreamCommandsServer{stream})
}

var agentServiceDesc = grpc.ServiceDesc{
	ServiceName: "agent.v1.AgentService",
	HandlerType: (*AgentServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamCommands",
			Handler:       _AgentService_StreamCommands_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
}

// RegisterAgentServiceServer registers the server implementation with a gRPC server.
func RegisterAgentServiceServer(s grpc.ServiceRegistrar, srv AgentServiceServer) {
	s.RegisterService(&agentServiceDesc, srv)
}

// AgentServiceClient is the client stub used by the agent.
type AgentServiceClient interface {
	StreamCommands(ctx context.Context, opts ...grpc.CallOption) (AgentService_StreamCommandsClient, error)
}

// AgentService_StreamCommandsClient is the client-side stream handle.
type AgentService_StreamCommandsClient interface {
	Send(*AgentMessage) error
	Recv() (*AgentMessage, error)
	grpc.ClientStream
}

type agentServiceClient struct{ cc grpc.ClientConnInterface }

// NewAgentServiceClient creates a new client stub.
func NewAgentServiceClient(cc grpc.ClientConnInterface) AgentServiceClient {
	return &agentServiceClient{cc}
}

type agentServiceStreamCommandsClient struct{ grpc.ClientStream }

func (x *agentServiceStreamCommandsClient) Send(m *AgentMessage) error {
	return x.ClientStream.SendMsg(m)
}
func (x *agentServiceStreamCommandsClient) Recv() (*AgentMessage, error) {
	m := new(AgentMessage)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *agentServiceClient) StreamCommands(ctx context.Context, opts ...grpc.CallOption) (AgentService_StreamCommandsClient, error) {
	stream, err := c.cc.NewStream(ctx, &grpc.StreamDesc{
		ServerStreams: true,
		ClientStreams: true,
	}, streamCommandsMethod, opts...)
	if err != nil {
		return nil, err
	}
	return &agentServiceStreamCommandsClient{stream}, nil
}
