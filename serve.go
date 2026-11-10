package gateway

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/mlund01/squadron-gateway-sdk/proto"
)

// MaxGRPCMessageBytes limits each encoded gRPC message to 32 MiB, allowing
// file attachments to travel with the message payload.
const MaxGRPCMessageBytes = 32 << 20 // 32 MiB

// Serve is the entry point for a gateway binary. Call it from main()
// after constructing the gateway implementation. Serve blocks until
// squadron tears down the subprocess.
//
// Gateway authors do not need to know anything about hashicorp/go-plugin
// or gRPC — the SDK handles handshake, broker setup, and the bidirectional
// connection to squadron's SquadronAPI service.
func Serve(impl Gateway) {
	go monitorParent()

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshake(),
		Plugins: map[string]plugin.Plugin{
			PluginName: &gatewayPluginImpl{Impl: impl},
		},
		GRPCServer: func(opts []grpc.ServerOption) *grpc.Server {
			opts = append(opts,
				grpc.MaxRecvMsgSize(MaxGRPCMessageBytes),
				grpc.MaxSendMsgSize(MaxGRPCMessageBytes),
			)
			return grpc.NewServer(opts...)
		},
	})
}

func handshake() plugin.HandshakeConfig {
	return plugin.HandshakeConfig{
		ProtocolVersion:  HandshakeProtocolVersion,
		MagicCookieKey:   HandshakeMagicCookieKey,
		MagicCookieValue: HandshakeMagicCookieValue,
	}
}

// gatewayPluginImpl registers the gateway service with go-plugin.
type gatewayPluginImpl struct {
	plugin.Plugin
	Impl Gateway
}

func (p *gatewayPluginImpl) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterGatewayServiceServer(s, &gatewayServer{
		impl:   p.Impl,
		broker: broker,
	})
	return nil
}

func (p *gatewayPluginImpl) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCGatewayClient{
		client: pb.NewGatewayServiceClient(c),
		broker: broker,
	}, nil
}

// gatewayServer is the gRPC adapter the plugin process registers. It
// translates gRPC calls into the Gateway interface and lazily wires up
// the SquadronAPI client when Configure runs.
type gatewayServer struct {
	pb.UnimplementedGatewayServiceServer
	impl   Gateway
	broker *plugin.GRPCBroker

	apiOnce sync.Once
	api     SquadronAPI
}

func (s *gatewayServer) Configure(ctx context.Context, req *pb.ConfigureRequest) (*pb.ConfigureResponse, error) {
	if req.BrokerStreamId == 0 {
		return &pb.ConfigureResponse{
			Success: false,
			Error:   "configure: broker_stream_id is required",
		}, nil
	}

	// Dial squadron's SquadronAPI service. Cached for the rest of the
	// gateway's lifetime — the squadron host serves the broker stream
	// for the lifetime of the subprocess connection.
	var dialErr error
	s.apiOnce.Do(func() {
		conn, err := s.broker.Dial(req.BrokerStreamId)
		if err != nil {
			dialErr = fmt.Errorf("dial squadron api: %w", err)
			return
		}
		s.api = &squadronAPIClient{client: pb.NewSquadronServiceClient(conn)}
	})
	if dialErr != nil {
		return &pb.ConfigureResponse{Success: false, Error: dialErr.Error()}, nil
	}

	if err := s.impl.Configure(ctx, req.Settings, s.api); err != nil {
		return &pb.ConfigureResponse{Success: false, Error: err.Error()}, nil
	}
	return &pb.ConfigureResponse{Success: true}, nil
}

func (s *gatewayServer) OnHumanInputRequested(ctx context.Context, p *pb.HumanInputRecord) (*pb.Empty, error) {
	if err := s.impl.OnHumanInputRequested(ctx, recordFromProto(p)); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (s *gatewayServer) OnHumanInputResolved(ctx context.Context, p *pb.HumanInputRecord) (*pb.Empty, error) {
	if err := s.impl.OnHumanInputResolved(ctx, recordFromProto(p)); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (s *gatewayServer) OnNotification(ctx context.Context, p *pb.NotificationRecord) (*pb.Empty, error) {
	if err := s.impl.OnNotification(ctx, notificationFromProto(p)); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (s *gatewayServer) PostMessage(ctx context.Context, p *pb.PostMessageRequest) (*pb.PostMessageResponse, error) {
	resp, err := s.impl.PostMessage(ctx, postMessageFromProto(p))
	if err != nil {
		return nil, err
	}
	return &pb.PostMessageResponse{MessageId: resp.MessageID}, nil
}

func (s *gatewayServer) MessageToolSpec(ctx context.Context, _ *pb.Empty) (*pb.MessageToolSpecResponse, error) {
	spec, err := s.impl.MessageToolSpec(ctx)
	if err != nil {
		return nil, err
	}
	return messageToolSpecToProto(spec), nil
}

func (s *gatewayServer) EditMessage(ctx context.Context, p *pb.EditMessageRequest) (*pb.Empty, error) {
	editor, ok := s.impl.(Editor)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "this gateway doesn't edit messages")
	}
	if err := editor.EditMessage(ctx, editMessageFromProto(p)); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (s *gatewayServer) Shutdown(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	if err := s.impl.Shutdown(ctx); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// squadronAPIClient is the gateway-side adapter that turns a
// SquadronService gRPC client into the SquadronAPI Go interface.
type squadronAPIClient struct {
	client pb.SquadronServiceClient
}

func (c *squadronAPIClient) ListHumanInputs(ctx context.Context, filter HumanInputFilter) ([]HumanInputRecord, int, error) {
	resp, err := c.client.ListHumanInputs(ctx, filterToProto(filter))
	if err != nil {
		return nil, 0, err
	}
	out := make([]HumanInputRecord, 0, len(resp.Items))
	for _, item := range resp.Items {
		out = append(out, recordFromProto(item))
	}
	return out, int(resp.Total), nil
}

func (c *squadronAPIClient) ResolveHumanInput(ctx context.Context, toolCallID, response, responderUserID string) (ResolveResult, error) {
	resp, err := c.client.ResolveHumanInput(ctx, &pb.ResolveHumanInputRequest{
		ToolCallId:      toolCallID,
		Response:        response,
		ResponderUserId: responderUserID,
	})
	if err != nil {
		return ResolveResult{}, err
	}
	return ResolveResult{
		Record:          recordFromProto(resp.Record),
		AlreadyResolved: resp.AlreadyResolved,
		NotFound:        resp.NotFound,
	}, nil
}
