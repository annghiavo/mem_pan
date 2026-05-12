FROM golang:1.26.2-alpine AS builder
WORKDIR /workspace
COPY . .
WORKDIR /workspace/services/admin-service
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /bin/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/server /server
EXPOSE 8083 9093
ENTRYPOINT ["/server"]
