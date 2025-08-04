package usecase

import (
	"GoRoutine/internal/cache"
	"GoRoutine/internal/interfaces"
)

type UseCases struct {
	interfaces.ProcessUsecase
}

func NewUsecases(c *cache.ProcessManager) interfaces.Usecases {

	return &UseCases{
		NewProcessUsecase(c),
	}

}
