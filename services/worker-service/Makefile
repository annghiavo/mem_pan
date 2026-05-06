-include app.env

export SERVER_ADDRESS \
	GRPC_SERVER_ADDRESS HTTP_SERVER_ADDRESS

.PHONY: run test

run:
	go run cmd/server/main.go

test:
	go test -v -cover ./...
