package interfaces

import (
	"GoRoutine/internal/domain/entities"

	"github.com/gofrs/uuid"
)

type Usecases interface {
	ProcessUsecase
}

type ProcessUsecase interface {
	StartProcess() (uuid.UUID, error)
	StartProcessWithFile(filePath string, numSpeakers int, vadThreshold float64) (uuid.UUID, error)
	GetStatus(id uuid.UUID) (*entities.ProcessStatus, bool)
	GetAllProcessIDs() []uuid.UUID
	WaitForCompletion(id uuid.UUID) *entities.ProcessStatus
	SaveAIResult(id uuid.UUID, result []entities.AIResult)
}
