{ pkgs, ... }: {
  channel = "stable-24.05";

  packages = [
    # core
    pkgs.go
    pkgs.gnumake

    # docker
    pkgs.docker
    pkgs.docker-compose

    # database tools
    pkgs.migrate
    pkgs.sqlc

    # testing
    pkgs.mockgen

    # protobuf + grpc
    pkgs.protobuf
    pkgs.protoc-gen-go
    pkgs.protoc-gen-go-grpc
    pkgs.grpc-gateway
    

    # grpc client
    pkgs.evans
  ];

  services.docker.enable = true;

  env = {};

  idx = {
    extensions = [
      "golang.go"
    ];
  };
}