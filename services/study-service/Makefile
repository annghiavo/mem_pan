-include app.env

DB_URL ?= $(if $(strip $(DIRECT_URL)),$(DIRECT_URL),$(DATABASE_URL))
export DB_URL DIRECT_URL DATABASE_URL \
	GRPC_SERVER_ADDRESS HTTP_SERVER_ADDRESS \
	AUTH_SERVICE_ADDRESS DECK_SERVICE_ADDRESS

PROTO_DIR   := proto
THIRD_PARTY := ../deck-service/third_party
PB_OUT      := pb
SWAGGER_OUT := doc/swagger

COVER_OUT  := coverage.out
COVER_HTML := coverage.html

.PHONY: proto sqlc migrateup migrateup1 migratedown migratedown1 run evans \
        test test/cover test/html test/clean

proto:
	mkdir -p $(PB_OUT) $(SWAGGER_OUT)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--proto_path=$(THIRD_PARTY) \
		--go_out=$(PB_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(PB_OUT) --go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=$(PB_OUT) --grpc-gateway_opt=paths=source_relative \
		--openapiv2_out=$(SWAGGER_OUT) \
		--openapiv2_opt=allow_merge=true,merge_file_name=study_service \
		$(PROTO_DIR)/*.proto

sqlc:
	cd db && sqlc generate

migrateup:
	@$(MAKE) check-db-url
	migrate -path db/migration -database "$(DB_URL)" -verbose up

migrateup1:
	@$(MAKE) check-db-url
	migrate -path db/migration -database "$(DB_URL)" -verbose up 1

migratedown:
	@$(MAKE) check-db-url
	migrate -path db/migration -database "$(DB_URL)" -verbose down

migratedown1:
	@$(MAKE) check-db-url
	migrate -path db/migration -database "$(DB_URL)" -verbose down 1

check-db-url:
	@test -n "$(DB_URL)" || (echo "DB_URL, DIRECT_URL, or DATABASE_URL is required" && exit 1)

run:
	go run cmd/server/main.go

evans:
	evans --host localhost --port 9092 -r repl

test:
	go test -v -cover -short ./...

test/cover:
	go test -v -cover -short ./... -coverprofile=$(COVER_OUT) -covermode=atomic
	go tool cover -func=$(COVER_OUT)

test/html: test/cover
	go tool cover -html=$(COVER_OUT) -o $(COVER_HTML)
	@echo "Coverage report written to $(COVER_HTML)"

test/clean:
	rm -f $(COVER_OUT) $(COVER_HTML)
