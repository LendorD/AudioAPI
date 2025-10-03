package interfaces

import (
	"GoRoutine/internal/domain/entities"

	"github.com/gofrs/uuid"
)

type Usecases interface {
	// ProcessUsecase
	ProcessUsecaseAI
}

// type ProcessUsecase interface {
// 	StartProcess() (uuid.UUID, error)
// 	StartProcessWithFile(filePath string, numSpeakers int, vadThreshold float64) (uuid.UUID, error)
// 	GetStatus(id uuid.UUID) (*entities.ProcessStatus, bool)
// 	GetAllProcessIDs() []uuid.UUID
// 	WaitForCompletion(id uuid.UUID) *entities.ProcessStatus
// 	SaveDealAnalysisResult(id uuid.UUID, result []entities.AIResult) error
// }

type ProcessUsecaseAI interface {
	// StartFullProcess(numSpeakers int, vadThreshold float64) (uuid.UUID, error)
	StartProcessWithFileAI(filePath string, numSpeakers int, vadThreshold float64) (uuid.UUID, error)
	StartDetailProcessWithFile(filePath string, numSpeakers int, vadThreshold float64) (uuid.UUID, error)
	SaveToxicityAnalysisResult(id uuid.UUID, result entities.ToxicityAnalysis) error
	WaitForCompletion(id uuid.UUID) *entities.ProcessStatus
	GetStatus(id uuid.UUID) (*entities.ProcessStatus, bool)
	GetAllProcessIDs() []uuid.UUID
	GetHighToxicityProcessesFromDB() ([]entities.ProcessWithFile, error)
	SaveDealAnalysisResult(id uuid.UUID, result []entities.AIResult) error
}
