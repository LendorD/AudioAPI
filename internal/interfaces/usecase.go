package interfaces

import (
	"GoRoutine/internal/domain/entities"

	"github.com/gofrs/uuid"
)

type Usecases interface {
	ProcessUsecase
}

type ProcessUsecase interface {
	StartProcess() uuid.UUID
	StartProcessWithFile(filePath string, numSpeakers int, vadThreshold float64) uuid.UUID
	GetStatus(id uuid.UUID) (*entities.ProcessStatus, bool)
	GetAllProcessIDs() []uuid.UUID
}
