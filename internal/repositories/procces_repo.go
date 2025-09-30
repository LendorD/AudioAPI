package repositories

import (
	"GoRoutine/internal/domain/entities"
	"GoRoutine/internal/interfaces"
	"fmt"
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type ProcessRecord struct {
	ID               uuid.UUID      `gorm:"primaryKey;type:uuid"`
	FileName         string         `gorm:"not null"`
	RawText          string         `gorm:"not null"`
	ToxicityAnalysis ToxicityResult `gorm:"type:jsonb"`
	StartedAt        time.Time      `gorm:"not null"`
	FinishedAt       *time.Time     `gorm:"default:null"`
	CreatedAt        time.Time      `gorm:"autoCreateTime"`
}

type ToxicityResult entities.ToxicityAnalysis

type proccesRepository struct {
	db *gorm.DB
}

func NewProccesRepository(db *gorm.DB) interfaces.ProccesRepository {
	// Автомиграция
	// db.AutoMigrate(&ProcessRecord{})
	return &proccesRepository{db: db}
}

func (r *proccesRepository) SaveProcess(id uuid.UUID, v2 *entities.ProcessStatusV2) error {
	if v2.DataRaw == nil {
		return fmt.Errorf("DataRaw is nil")
	}

	record := ProcessRecord{
		ID:         id,
		FileName:   v2.FileName,
		RawText:    v2.DataRaw.Content,
		StartedAt:  v2.StartedAt,
		FinishedAt: v2.FinishedAt,
	}

	return r.db.Create(&record).Error
}

func (r *proccesRepository) UpdateToxicityAnalysis(id uuid.UUID, analysis entities.ToxicityAnalysis) error {
	return r.db.Model(&ProcessRecord{}).
		Where("id = ?", id).
		Update("toxicity_analysis", ToxicityResult(analysis)).
		Error
}
