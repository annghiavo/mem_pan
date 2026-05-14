-include app.env
export GRPC_SERVER_ADDRESS HTTP_SERVER_ADDRESS AUTH_SERVICE_ADDRESS \
	ELASTICSEARCH_URL ELASTICSEARCH_API_KEY PUBSUB_PUSH_SECRET \
	ES_DECK_INDEX ES_FOLDER_INDEX ES_CARD_INDEX ES_USER_INDEX

PROTO_DIR   := proto
THIRD_PARTY := third_party
PB_OUT      := pb
SWAGGER_OUT := doc/swagger

.PHONY: proto run evans test test/cover

proto:
	mkdir -p $(PB_OUT) $(SWAGGER_OUT)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--proto_path=$(THIRD_PARTY) \
		--go_out=$(PB_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(PB_OUT) --go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=$(PB_OUT) --grpc-gateway_opt=paths=source_relative \
		--openapiv2_out=$(SWAGGER_OUT) \
		--openapiv2_opt=allow_merge=true,merge_file_name=search_service \
		$(PROTO_DIR)/*.proto

run:
	go run cmd/server/main.go

evans:
	evans --host localhost --port 9096 -r repl

COVER_OUT := coverage.out

test:
	go test -v -cover -short ./...

test/cover:
	go test -v -cover -short ./... -coverprofile=$(COVER_OUT) -covermode=atomic
	go tool cover -func=$(COVER_OUT)
