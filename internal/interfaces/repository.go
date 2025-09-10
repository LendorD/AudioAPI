package interfaces

import (
	"GoRoutine/internal/domain/entities"

	"github.com/gofrs/uuid"
)

type ProccesRepository interface {
	SaveProcess(id uuid.UUID, v2 *entities.ProcessStatusV2) error
	UpdateToxicityAnalysis(id uuid.UUID, analysis entities.ToxicityAnalysis) error
}
