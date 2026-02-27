package grpc

import (
	"context"
	"log/slog"
	"quantlo/internal/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GrpcBus publishes events to a remote EventService over gRPC.
// Used when BusProvider == "grpc" in config.
type GrpcBus struct {
	conn   *grpc.ClientConn
	client proto.EventServiceClient
	events chan *proto.EventRequest
}

// NewGrpcBusFromAddr dials the remote EventService and returns a GrpcBus and a cleanup function.
func NewGrpcBusFromAddr(addr string, bufferSize int) (*GrpcBus, func(), error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	client := proto.NewEventServiceClient(conn)
	events := make(chan *proto.EventRequest, bufferSize)

	bus := &GrpcBus{
		conn:   conn,
		client: client,
		events: events,
	}

	go bus.worker()

	cleanup := func() {
		close(events)
		_ = conn.Close()
	}
	return bus, cleanup, nil
}

func (b *GrpcBus) worker() {
	for req := range b.events {
		// Use a fresh context for each publish since it's background
		_, err := b.client.Publish(context.Background(), req)
		if err != nil {
			slog.Error("grpc bus: async publish failed", "topic", req.Topic, "error", err)
		}
	}
}

// Publish sends an event to the remote EventService asynchronously.
func (b *GrpcBus) Publish(topic string, data []byte) error {
	select {
	case b.events <- &proto.EventRequest{
		Topic:   topic,
		Payload: data,
	}:
		return nil
	default:
		slog.Warn("grpc bus: buffer full, dropping event", "topic", topic)
		return nil
	}
}
