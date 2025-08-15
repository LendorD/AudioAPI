package usecase

import (
	"GoRoutine/internal/cache"
	"GoRoutine/internal/config"
	"GoRoutine/internal/interfaces"
)

type UseCases struct {
	interfaces.ProcessUsecase
}

func NewUsecases(c *cache.ProcessManager, cfg *config.Config) interfaces.Usecases {
	return &UseCases{
		ProcessUsecase: NewProcessUsecase(c, cfg),
	}
}
