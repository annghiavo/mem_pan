package gapi

import (
	"mem_pan/services/search-service/internal/authclient"
	"mem_pan/services/search-service/internal/service"
	pb "mem_pan/services/search-service/pb"
)

type Server struct {
	pb.UnimplementedSearchServiceServer
	svc        service.SearchService
	authClient authclient.Client
}

func NewServer(svc service.SearchService, authClient authclient.Client) *Server {
	return &Server{svc: svc, authClient: authClient}
}
