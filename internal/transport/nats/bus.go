package nats

import "github.com/nats-io/nats.go"

type Bus struct {
	nc *nats.Conn
}

func NewBus(nc *nats.Conn) *Bus {
	return &Bus{nc: nc}
}

func (b *Bus) Publish(topic string, data []byte) error {
	return b.nc.Publish(topic, data)
}