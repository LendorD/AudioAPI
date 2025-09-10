package usecase

import (
	"GoRoutine/internal/cache"
	"GoRoutine/internal/config"
	"GoRoutine/internal/interfaces"
)

type UseCases struct {
	interfaces.ProcessUsecase
	interfaces.ProcessUsecaseAI
}

func NewUsecases(r interfaces.ProccesRepository, c *cache.ProcessManager, cfg *config.Config) interfaces.Usecases {
	return &UseCases{
		ProcessUsecase:   NewProcessUsecase(c, cfg),
		ProcessUsecaseAI: NewProcessUsecaseAI(r, c, cfg),
	}
}
