package repository

type MessageBus interface {
	Publish(topic string, data []byte) error
}