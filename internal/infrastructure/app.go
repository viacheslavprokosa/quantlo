package infrastructure

import (
	"context"
	"golang.org/x/sync/errgroup"
)

type App struct {
	servers []Server
}

func NewApp(servers []Server) *App {
	return &App{servers: servers}
}

func (a *App) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	for _, srv := range a.servers {
		s := srv
		g.Go(func() error {
			return s.Start(ctx)
		})
	}

	<-ctx.Done()

	for _, srv := range a.servers {
		srv.Stop(context.Background())
	}

	return g.Wait()
}