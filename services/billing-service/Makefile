-include app.env

DB_URL ?= $(if $(strip $(DIRECT_URL)),$(DIRECT_URL),$(DATABASE_URL))
export DB_URL DIRECT_URL DATABASE_URL \
	GRPC_SERVER_ADDRESS HTTP_SERVER_ADDRESS AUTH_SERVICE_ADDRESS \
	PAYOS_CLIENT_ID PAYOS_API_KEY PAYOS_CHECKSUM_KEY PAYOS_BASE_URL \
	PAYOS_PAYOUT_CLIENT_ID PAYOS_PAYOUT_API_KEY PAYOS_PAYOUT_CHECKSUM_KEY \
	APP_BASE_URL PAYOS_RETURN_URL PAYOS_CANCEL_URL \
	PLUS_MONTHLY_AMOUNT_VND PLUS_YEARLY_AMOUNT_VND

PROTO_DIR := proto
PB_OUT    := pb

.PHONY: proto sqlc migrateup migrateup1 migratedown migratedown1 run test

proto:
	mkdir -p $(PB_OUT)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(PB_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(PB_OUT) --go-grpc_opt=paths=source_relative \
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
	go run ./cmd/server

test:
	go test -v -cover -short ./...
