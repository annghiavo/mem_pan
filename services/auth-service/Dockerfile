FROM golang:alpine AS builder
WORKDIR /workspace
COPY . .
WORKDIR /workspace/services/auth-service
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /bin/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/server /server
EXPOSE 8080 9090
ENTRYPOINT ["/server"]
