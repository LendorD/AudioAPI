package app

import (
	"GoRoutine/internal/cache"
	"GoRoutine/internal/config"
	"GoRoutine/internal/db"
	"GoRoutine/internal/handlers"
	"GoRoutine/internal/repositories"
	usecases "GoRoutine/internal/usecase"
	"context"
	"net/http"

	"go.uber.org/fx"
)

func New() *fx.App {
	return fx.New(
		fx.Provide(
			config.LoadConfig,
		),
		RepositoryModule,
		UsecaseModule,
		HttpServerModule,
		CacheModule,
	)
}
func InvokeHttpServer(lc fx.Lifecycle, h http.Handler) {
	server := &http.Server{
		Addr:    ":" + config.DefaultServerConfig().Port,
		Handler: h,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go server.ListenAndServe()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Close()
		},
	})
}

var HttpServerModule = fx.Module("http_server_module",
	fx.Provide(
		handlers.NewHandler,
		handlers.ProvideRouter,
	),
	fx.Invoke(InvokeHttpServer),
)

var RepositoryModule = fx.Module("postgres_module",
	fx.Provide(
		db.NewDB,
		repositories.NewProccesRepository,
	),
)

var UsecaseModule = fx.Module("usecases_module",
	fx.Provide(
		usecases.NewUsecases,
	),
	fx.Invoke(
		usecases.StartCleanupLifecycle,
	),
)

var CacheModule = fx.Module("cache_module",
	fx.Provide(
		cache.NewProcessCache,
	),
)
