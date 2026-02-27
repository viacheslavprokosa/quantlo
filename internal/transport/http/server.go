package http

import (
	"context"
	"net/http"
	"quantlo/internal/service"
	"time"
)

type Server struct {
	srv *http.Server
}

func NewServer(addr string, svc service.LedgerService) *Server {
	mux := http.NewServeMux()
	h := NewHandler(svc)
	h.Register(mux)

	return &Server{
		srv: &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
	}
}

func (s *Server) Start(ctx context.Context) error {
	return s.srv.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
