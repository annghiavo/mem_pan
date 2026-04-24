-include app.env

DB_URL ?= $(if $(strip $(DIRECT_URL)),$(DIRECT_URL),$(DATABASE_URL))

export DB_URL
export DIRECT_URL
export DATABASE_URL
export SERVER_ADDRESS

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

sqlc-all:
	for dir in services/*/db; do \
		echo "Generating $$dir..."; \
		(cd $$dir && sqlc generate); \
	done

test:
	go test -v -cover -short ./...

run:
	go run main.go

mock	:
	mockgen -package mockdb -destination db/mock/store.go mem_pan/db/sqlc Store

.PHONY: migrateup migrateup1 migratedown migratedown1 check-db-url sqlc-all test run mock