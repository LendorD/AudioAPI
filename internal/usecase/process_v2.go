package usecase

import (
	"GoRoutine/internal/cache"
	"GoRoutine/internal/config"
	"GoRoutine/internal/domain/entities"
	"GoRoutine/internal/interfaces"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gofrs/uuid"
)

type ProcessUsecaseAI struct {
	repo         interfaces.ProccesRepository
	Cache        *cache.ProcessManager
	Client       *http.Client
	PythonAPI    string
	MaxProcesses int
}

func NewProcessUsecaseAI(repo interfaces.ProccesRepository, c *cache.ProcessManager, cfg *config.Config) interfaces.ProcessUsecaseAI {
	maxProc, _ := strconv.Atoi(cfg.Server.MaxProcesses)
	return &ProcessUsecaseAI{
		repo:         repo,
		Cache:        c,
		Client:       &http.Client{},
		PythonAPI:    cfg.Server.PythonAPIURL,
		MaxProcesses: maxProc,
	}
}

func (uc *ProcessUsecaseAI) SaveToxicityAnalysisResult(id uuid.UUID, result entities.ToxicityAnalysis) error {
	if status, exists := uc.Cache.Get(id); exists {
		if v2, ok := status.Data.(*entities.ProcessStatusV2); ok {
			v2.AIResult = result
			uc.Cache.Set(id, status)
			if err := uc.repo.UpdateToxicityAnalysis(id, result); err != nil {
				log.Printf("Failed to update toxicity analysis in DB for %s: %v", id, err)
			} else {
				log.Printf("Toxicity analysis for %s updated in DB", id)
			}
			return nil
		}
	}
	return fmt.Errorf("process not found or not V2 type")
}

func (uc *ProcessUsecaseAI) WaitForCompletion(id uuid.UUID) *entities.ProcessStatus {
	for {
		status, exists := uc.Cache.Get(id)
		if !exists {
			return nil
		}

		// Проверяем, завершён ли процесс — смотрим внутрь Data
		if v1, ok := status.Data.(*entities.ProcessStatusV1); ok {
			if !v1.IsRunning {
				return status
			}
		} else if v2, ok := status.Data.(*entities.ProcessStatusV2); ok {
			if !v2.IsRunning {
				return status
			}
		}

		time.Sleep(10 * time.Second)
	}
}

func (uc *ProcessUsecaseAI) GetStatus(id uuid.UUID) (*entities.ProcessStatus, bool) {
	return uc.Cache.Get(id)
}

func (uc *ProcessUsecaseAI) GetAllProcessIDs() []uuid.UUID {
	return uc.Cache.GetAllProcessIDs()
}

func (uc *ProcessUsecaseAI) GetHighToxicityProcessesFromDB() ([]entities.ProcessWithFile, error) {
	return uc.repo.GetHighToxicityProcesses()
}

func (uc *ProcessUsecaseAI) SaveDealAnalysisResult(id uuid.UUID, result []entities.AIResult) error {
	if status, exists := uc.Cache.Get(id); exists {
		if v1, ok := status.Data.(*entities.ProcessStatusV1); ok {
			v1.AIResult = result
			uc.Cache.Set(id, status)
			return nil
		}
	}
	return fmt.Errorf("process not found or not V1 type")
}
