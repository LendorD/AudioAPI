package usecase

import (
	"GoRoutine/internal/cache"
	"GoRoutine/internal/config"
	"GoRoutine/internal/domain/entities"
	"GoRoutine/internal/interfaces"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
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

func NewProcessUsecaseAI(repo interfaces.ProccesRepository, c *cache.ProcessManager, cfg *config.Config) *ProcessUsecaseAI {
	maxProc, _ := strconv.Atoi(cfg.Server.MaxProcesses)
	return &ProcessUsecaseAI{
		repo:         repo,
		Cache:        c,
		Client:       &http.Client{},
		PythonAPI:    cfg.Server.PythonAPIURL,
		MaxProcesses: maxProc,
	}
}

func (uc *ProcessUsecaseAI) StartProcessWithFileAI(filePath string, numSpeakers int, vadThreshold float64) (uuid.UUID, error) {
	if uc.Cache.CountRunning() >= uc.MaxProcesses {
		return uuid.Nil, fmt.Errorf("max number of concurrent processes reached")
	}

	id, _ := uuid.NewV4()
	startTime := time.Now()

	// Создаём начальный статус V2
	initialStatus := &entities.ProcessStatusV2{
		ProcessStatusCommon: entities.ProcessStatusCommon{
			IsRunning: true,
			StartedAt: startTime,
		},
	}

	// Оборачиваем в универсальный ProcessStatus
	status := &entities.ProcessStatus{
		Type: "v2",
		Data: initialStatus,
	}

	uc.Cache.Set(id, status)

	go func(pid uuid.UUID) {
		// defer os.Remove(filePath)

		file, err := os.Open(filePath)
		if err != nil {
			errorStatus := &entities.ProcessStatusV2{
				ProcessStatusCommon: entities.ProcessStatusCommon{
					IsRunning: false,
					FileName:  filepath.Base(filePath),
					StartedAt: startTime,
				},
				DataRaw: &entities.AIContent{
					Content: "Failed to open file: " + err.Error(),
				},
			}
			uc.Cache.Set(pid, &entities.ProcessStatus{
				Type: "v2",
				Data: errorStatus,
			})
			return
		}
		defer file.Close()

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		partHeaders := textproto.MIMEHeader{}
		partHeaders.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "file", filepath.Base(filePath)))
		partHeaders.Set("Content-Type", "audio/mpeg")

		part, err := writer.CreatePart(partHeaders)
		if err != nil {
			errorStatus := &entities.ProcessStatusV2{
				ProcessStatusCommon: entities.ProcessStatusCommon{
					IsRunning: false,
					FileName:  filepath.Base(filePath),
					StartedAt: startTime,
				},
				DataRaw: &entities.AIContent{
					Content: "Failed to create multipart part: " + err.Error(),
				},
			}
			uc.Cache.Set(pid, &entities.ProcessStatus{
				Type: "v2",
				Data: errorStatus,
			})
			return
		}

		_, err = io.Copy(part, file)
		if err != nil {
			errorStatus := &entities.ProcessStatusV2{
				ProcessStatusCommon: entities.ProcessStatusCommon{
					IsRunning: false,
					FileName:  filepath.Base(filePath),
					StartedAt: startTime,
				},
				DataRaw: &entities.AIContent{
					Content: "Failed to copy file to request body: " + err.Error(),
				},
			}
			uc.Cache.Set(pid, &entities.ProcessStatus{
				Type: "v2",
				Data: errorStatus,
			})
			return
		}

		writer.Close()

		req, err := http.NewRequest("POST", "http://192.168.30.230:3001/api/v1/files/", &buf)
		if err != nil {
			errorStatus := &entities.ProcessStatusV2{
				ProcessStatusCommon: entities.ProcessStatusCommon{
					IsRunning: false,
					FileName:  filepath.Base(filePath),
					StartedAt: startTime,
				},
				DataRaw: &entities.AIContent{
					Content: "Failed to create request: " + err.Error(),
				},
			}
			uc.Cache.Set(pid, &entities.ProcessStatus{
				Type: "v2",
				Data: errorStatus,
			})
			return
		}

		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjkzMDEzZTUyLTkzMDMtNDFjOC04ZjAyLTZkYjM5NjMwMDY5MiJ9.h9U4nUMBX0RifEwIp8lf40G20Na66kBmCJy1Au25UaY")

		resp, err := uc.Client.Do(req)
		finishTime := time.Now()

		v2Data := &entities.ProcessStatusV2{
			ProcessStatusCommon: entities.ProcessStatusCommon{
				IsRunning:  false,
				FileName:   filepath.Base(filePath),
				StartedAt:  startTime,
				FinishedAt: &finishTime,
			},
		}

		if err != nil {
			v2Data.DataRaw = &entities.AIContent{
				Content: "Request failed: " + err.Error(),
			}
			uc.Cache.Set(pid, &entities.ProcessStatus{
				Type: "v2",
				Data: v2Data,
			})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			v2Data.DataRaw = &entities.AIContent{
				Content: "HTTP: " + fmt.Sprint(resp.StatusCode),
			}
			uc.Cache.Set(pid, &entities.ProcessStatus{
				Type: "v2",
				Data: v2Data,
			})
			return
		}

		var parsed struct {
			ID   string `json:"id"`
			Data struct {
				Content string `json:"content"`
			} `json:"data"`
		}

		log.Printf("Raw response body: %s", string(body))

		if err := json.Unmarshal(body, &parsed); err != nil {
			v2Data.DataRaw = &entities.AIContent{
				ID:      "",
				Content: fmt.Sprintf("Failed to parse JSON: %v\n%s", err, string(body)),
			}
		} else {
			v2Data.DataRaw = &entities.AIContent{
				ID:      parsed.ID,
				Content: parsed.Data.Content,
			}
		}

		uc.Cache.Set(pid, &entities.ProcessStatus{
			Type: "v2",
			Data: v2Data,
		})
		if err := uc.repo.SaveProcess(pid, v2Data); err != nil {
			log.Printf("Failed to save process to DB: %v", err)
		} else {
			log.Printf("Process %s saved to DB", pid)
		}
	}(id)

	return id, nil
}

func (uc *ProcessUsecaseAI) StartDetailProcessWithFile(filePath string, numSpeakers int, vadThreshold float64) (uuid.UUID, error) {
	if uc.Cache.CountRunning() >= uc.MaxProcesses {
		return uuid.Nil, fmt.Errorf("max number of concurrent processes reached")
	}

	id, _ := uuid.NewV4()
	startTime := time.Now()

	initialStatus := &entities.ProcessStatusV1{
		ProcessStatusCommon: entities.ProcessStatusCommon{
			IsRunning: true,
			StartedAt: startTime,
		},
	}

	status := &entities.ProcessStatus{
		Type: "v1",
		Data: initialStatus,
	}

	uc.Cache.Set(id, status)

	go func(pid uuid.UUID) {
		// defer os.Remove(filePath)

		file, err := os.Open(filePath)
		if err != nil {
			errorStatus := &entities.ProcessStatusV1{
				ProcessStatusCommon: entities.ProcessStatusCommon{
					IsRunning: false,
					FileName:  filepath.Base(filePath),
					StartedAt: startTime,
				},
				Data: []entities.AudioSegment{
					{Start: 0, End: 0, Speaker: "ERROR", Text: "Failed to open file: " + err.Error()},
				},
			}
			uc.Cache.Set(pid, &entities.ProcessStatus{
				Type: "v1",
				Data: errorStatus,
			})
			return
		}
		defer file.Close()

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		part, err := writer.CreateFormFile("file", filepath.Base(filePath))
		if err == nil {
			_, _ = io.Copy(part, file)
		}

		_ = writer.WriteField("speakers", fmt.Sprintf("%d", numSpeakers))
		_ = writer.WriteField("accuracy", fmt.Sprintf("%f", vadThreshold))

		writer.Close()

		req, err := http.NewRequest("POST", uc.PythonAPI, &buf)
		if err != nil {
			errorStatus := &entities.ProcessStatusV1{
				ProcessStatusCommon: entities.ProcessStatusCommon{
					IsRunning: false,
					FileName:  filepath.Base(filePath),
					StartedAt: startTime,
				},
				Data: []entities.AudioSegment{
					{Start: 0, End: 0, Speaker: "ERROR", Text: "Failed to create request: " + err.Error()},
				},
			}
			uc.Cache.Set(pid, &entities.ProcessStatus{
				Type: "v1",
				Data: errorStatus,
			})
			return
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := uc.Client.Do(req)

		finishTime := time.Now()

		v1Data := &entities.ProcessStatusV1{
			ProcessStatusCommon: entities.ProcessStatusCommon{
				IsRunning:  false,
				FileName:   filepath.Base(filePath),
				StartedAt:  startTime,
				FinishedAt: &finishTime,
			},
		}

		if err != nil {
			v1Data.Data = []entities.AudioSegment{
				{Start: 0, End: 0, Speaker: "ERROR", Text: "Request failed: " + err.Error()},
			}
			uc.Cache.Set(pid, &entities.ProcessStatus{
				Type: "v1",
				Data: v1Data,
			})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			status.Data = []entities.AudioSegment{
				{Start: 0, End: 0, Speaker: "ERROR", Text: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))},
			}
			uc.Cache.Set(pid, &entities.ProcessStatus{
				Type: "v1",
				Data: v1Data,
			})
			return
		}

		var parsed entities.PythonAPIResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			status.Data = []entities.AudioSegment{
				{
					Start:   0,
					End:     0,
					Speaker: "ERROR",
					Text:    fmt.Sprintf("Failed to parse JSON: %v\n%s", err, string(body)),
				},
			}
		} else {
			v1Data.Data = parsed.Results.Result
		}

		uc.Cache.Set(pid, &entities.ProcessStatus{
			Type: "v1",
			Data: v1Data,
		})
	}(id)

	return id, nil
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
