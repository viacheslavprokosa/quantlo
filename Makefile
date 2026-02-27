.PHONY: proto build run

proto:
	buf generate api/proto

build: proto
	go build -o bin/api cmd/api/main.go

run: build
	./bin/api

clean:
	rm -rf internal/proto/*.pb.go
	rm -rf bin/