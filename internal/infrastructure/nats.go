package infrastructure

import (
	"github.com/nats-io/nats.go"
)

func connectNats(url string) (*nats.Conn, error) {
	if url == "" {
		return nil, nil
	}
	
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}

	return nc, nil
}