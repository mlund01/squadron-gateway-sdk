package gateway

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pb "github.com/mlund01/squadron-gateway-sdk/proto"
)

// Host-facing types. Squadron imports these to drive a loaded gateway
// subprocess. Gateway authors should not need any of this — they only
// implement Gateway and call Serve.

// HostPlugin is the variant of the gRPC plugin used by the host
// (squadron) to talk to a launched gateway subprocess. Construct it
// with the SquadronAPI implementation you want the gateway to call
// back into; the host side automatically registers that on a broker
// stream and passes the stream id to the gateway in Configure.
type HostPlugin struct {
	plugin.Plugin
	// API is squadron's implementation of the SquadronAPI surface. The
	// host plugin registers it on a broker stream and hands the stream
	// id to the gateway during Configure so the gateway can dial back.
	API SquadronAPI
}

// HostHandshake returns the handshake config the host should use when
// constructing a plugin.Client. Symmetric with the value the gateway
// uses on Serve — mismatched values prevent the connection.
func HostHandshake() plugin.HandshakeConfig {
	return handshake()
}

// PluginMap returns the plugin map for plugin.NewClient. Always pairs
// PluginName with this HostPlugin so squadron and the gateway agree on
// the registration key.
func (h *HostPlugin) PluginMap() map[string]plugin.Plugin {
	return map[string]plugin.Plugin{PluginName: h}
}

func (h *HostPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// The host doesn't serve GatewayService; that's the plugin's job.
	// We satisfy the interface so plugin.Plugin is happy on both sides.
	return nil
}

func (h *HostPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	streamID := broker.NextId()

	// Serve SquadronService on the chosen broker stream. AcceptAndServe
	// blocks; goroutine it. The blocking call returns when the
	// connection tears down, which mirrors the subprocess lifetime.
	go broker.AcceptAndServe(streamID, func(opts []grpc.ServerOption) *grpc.Server {
		s := grpc.NewServer(opts...)
		pb.RegisterSquadronServiceServer(s, &squadronServiceServer{api: h.API})
		return s
	})

	client := pb.NewGatewayServiceClient(c)
	return &GRPCGatewayClient{
		client:           client,
		broker:           broker,
		brokerStreamID:   streamID,
		hostManagedStream: true,
	}, nil
}

// GRPCGatewayClient is the host-side handle on a running gateway. It
// is returned from plugin.NewClient(...).Client().Dispense(...) and
// satisfies the GatewayClient interface that squadron uses.
type GRPCGatewayClient struct {
	client            pb.GatewayServiceClient
	broker            *plugin.GRPCBroker
	brokerStreamID    uint32
	hostManagedStream bool
}

// Configure pushes settings to the gateway and tells it the broker
// stream id where SquadronService is being served. After this returns
// successfully the gateway has a working SquadronAPI client and is
// ready to receive event pushes.
func (g *GRPCGatewayClient) Configure(ctx context.Context, settings map[string]string) error {
	if !g.hostManagedStream || g.brokerStreamID == 0 {
		return fmt.Errorf("configure: broker stream not initialized — host plugin not fully wired")
	}
	resp, err := g.client.Configure(ctx, &pb.ConfigureRequest{
		Settings:       settings,
		BrokerStreamId: g.brokerStreamID,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("gateway configure: %s", resp.Error)
	}
	return nil
}

// OnHumanInputRequested forwards a new request event to the gateway.
func (g *GRPCGatewayClient) OnHumanInputRequested(ctx context.Context, rec HumanInputRecord) error {
	_, err := g.client.OnHumanInputRequested(ctx, recordToProto(rec))
	return err
}

// OnHumanInputResolved forwards a resolution event to the gateway.
func (g *GRPCGatewayClient) OnHumanInputResolved(ctx context.Context, rec HumanInputRecord) error {
	_, err := g.client.OnHumanInputResolved(ctx, recordToProto(rec))
	return err
}

// OnNotification forwards a mission-lifecycle notification to the gateway.
func (g *GRPCGatewayClient) OnNotification(ctx context.Context, rec NotificationRecord) error {
	_, err := g.client.OnNotification(ctx, notificationToProto(rec))
	return err
}

// Shutdown asks the gateway to clean up before squadron kills the
// subprocess. Best-effort — squadron should tear down the subprocess
// regardless of return value.
func (g *GRPCGatewayClient) Shutdown(ctx context.Context) error {
	_, err := g.client.Shutdown(ctx, &pb.Empty{})
	return err
}

// squadronServiceServer is the gRPC adapter on the host side that
// translates incoming SquadronService calls from a gateway into the
// host's SquadronAPI implementation.
type squadronServiceServer struct {
	pb.UnimplementedSquadronServiceServer
	api SquadronAPI
}

func (s *squadronServiceServer) ListHumanInputs(ctx context.Context, p *pb.HumanInputFilter) (*pb.HumanInputList, error) {
	if s.api == nil {
		return nil, fmt.Errorf("squadron api not configured")
	}
	rows, total, err := s.api.ListHumanInputs(ctx, filterFromProto(p))
	if err != nil {
		return nil, err
	}
	out := &pb.HumanInputList{Items: make([]*pb.HumanInputRecord, 0, len(rows)), Total: int32(total)}
	for _, r := range rows {
		out.Items = append(out.Items, recordToProto(r))
	}
	return out, nil
}

func (s *squadronServiceServer) ResolveHumanInput(ctx context.Context, p *pb.ResolveHumanInputRequest) (*pb.ResolveHumanInputResponse, error) {
	if s.api == nil {
		return nil, fmt.Errorf("squadron api not configured")
	}
	res, err := s.api.ResolveHumanInput(ctx, p.ToolCallId, p.Response, p.ResponderUserId)
	if err != nil {
		return nil, err
	}
	return &pb.ResolveHumanInputResponse{
		Record:          recordToProto(res.Record),
		AlreadyResolved: res.AlreadyResolved,
		NotFound:        res.NotFound,
	}, nil
}
