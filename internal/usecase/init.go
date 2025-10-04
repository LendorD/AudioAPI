package usecase

import (
	"GoRoutine/internal/cache"
	"GoRoutine/internal/config"
	"GoRoutine/internal/interfaces"
)

type UseCases struct {
	interfaces.ProcessUsecaseAI
	interfaces.ThemeUsecase
	interfaces.ToxicUsecase
}

func NewUsecases(r interfaces.ProccesRepository, c *cache.ProcessManager, cfg *config.Config) interfaces.Usecases {
	return &UseCases{
		ProcessUsecaseAI: NewProcessUsecaseAI(r, c, cfg),
		ThemeUsecase:     NewThemeUsecase(r, c, cfg),
		ToxicUsecase:     NewToxicUsecase(r, c, cfg),
	}
}
