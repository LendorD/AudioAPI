package usecase

import (
	"GoRoutine/internal/cache"
	"context"
	"log"
	"time"

	"go.uber.org/fx"
)

func StartCleanupLifecycle(lc fx.Lifecycle, cache *cache.ProcessManager) {
	var stopCleanup context.CancelFunc

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			var ctx context.Context
			ctx, stopCleanup = context.WithCancel(context.Background())

			go func() {
				ticker := time.NewTicker(1 * time.Hour)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						log.Println("Cleanup goroutine started")
						cache.CleanupOldProcesses()
					case <-ctx.Done():
						log.Println("Cleanup goroutine stopped")
						return
					}
				}
			}()

			return nil
		},
		OnStop: func(context.Context) error {
			stopCleanup()
			return nil
		},
	})
}
