package grpc

import (
	"context"
	"quantlo/internal/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GrpcBus publishes events to a remote EventService over gRPC.
// Used when BusProvider == "grpc" in config.
type GrpcBus struct {
	conn   *grpc.ClientConn
	client proto.EventServiceClient
}

// NewGrpcBusFromAddr dials the remote EventService and returns a GrpcBus and a cleanup function.
func NewGrpcBusFromAddr(addr string) (*GrpcBus, func(), error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	client := proto.NewEventServiceClient(conn)
	cleanup := func() { conn.Close() }
	return &GrpcBus{conn: conn, client: client}, cleanup, nil
}

// Publish sends an event to the remote EventService.
func (b *GrpcBus) Publish(topic string, data []byte) error {
	_, err := b.client.Publish(context.Background(), &proto.EventRequest{
		Topic:   topic,
		Payload: data,
	})
	return err
}
