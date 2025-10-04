package interfaces

import (
	"GoRoutine/internal/domain/entities"

	"github.com/gofrs/uuid"
)

type Usecases interface {
	ProcessUsecaseAI
	ToxicUsecase
	ThemeUsecase
}

type ProcessUsecaseAI interface {
	SaveToxicityAnalysisResult(id uuid.UUID, result entities.ToxicityAnalysis) error
	WaitForCompletion(id uuid.UUID) *entities.ProcessStatus
	GetStatus(id uuid.UUID) (*entities.ProcessStatus, bool)
	GetAllProcessIDs() []uuid.UUID
	GetHighToxicityProcessesFromDB() ([]entities.ProcessWithFile, error)
	SaveDealAnalysisResult(id uuid.UUID, result []entities.AIResult) error
}

type ToxicUsecase interface {
	StartProcessWithFileAI(filePath string, numSpeakers int, vadThreshold float64) (uuid.UUID, error)
}

type ThemeUsecase interface {
	StartDetailProcessWithFile(filePath string, numSpeakers int, vadThreshold float64) (uuid.UUID, error)
}
