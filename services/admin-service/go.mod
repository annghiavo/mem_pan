module mem_pan/services/admin-service

go 1.25.5

replace mem_pan/pkg => ../../pkg

require (
	github.com/google/uuid v1.6.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0
	github.com/lib/pq v1.12.3
	github.com/sqlc-dev/pqtype v0.3.0
	google.golang.org/genproto/googleapis/api v0.0.0-20260504160031-60b97b32f348
	google.golang.org/grpc v1.81.0
	google.golang.org/protobuf v1.36.11
)

require (
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260427160629-7cedc36a6bc4 // indirect
)
