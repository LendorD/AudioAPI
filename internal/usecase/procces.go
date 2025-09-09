package usecase

import (
	"GoRoutine/internal/cache"
	"GoRoutine/internal/config"
	"GoRoutine/internal/domain/entities"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gofrs/uuid"
)

type ProcessUsecase struct {
	Cache        *cache.ProcessManager
	Client       *http.Client
	PythonAPI    string
	MaxProcesses int
}

func NewProcessUsecase(c *cache.ProcessManager, cfg *config.Config) *ProcessUsecase {
	maxProc, _ := strconv.Atoi(cfg.Server.MaxProcesses)
	return &ProcessUsecase{
		Cache:        c,
		Client:       &http.Client{},
		PythonAPI:    cfg.Server.PythonAPIURL,
		MaxProcesses: maxProc,
	}
}

func (uc *ProcessUsecase) StartProcess() (uuid.UUID, error) {
	maxProcesses := uc.MaxProcesses

	if uc.Cache.CountRunning() >= maxProcesses {
		return uuid.Nil, fmt.Errorf("max number of concurrent processes reached")
	}
	// Генерация UUID
	id, _ := uuid.NewV4()

	startTime := time.Now()

	arg1 := "test.mp3"

	// Сохраняем процесс как запущенный
	uc.Cache.Set(id, &entities.ProcessStatus{
		IsRunning: true,
		StartedAt: startTime,
	})

	go func(pid uuid.UUID) {
		//					"python"
		cmd := exec.Command("python", "python-scripts/script.py", arg1)

		// отключаем буферизацию вывода
		// cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")

		// cmd.Env = append(os.Environ(),
		// 	"PATH=C:\\Users\\dlucenko\\Desktop\\AudioAPI\\AudioAPI\\venv\\Scripts;"+os.Getenv("PATH"))

		out, err := cmd.CombinedOutput()
		finishTime := time.Now()

		status := &entities.ProcessStatus{
			IsRunning:  false,
			StartedAt:  startTime,
			FinishedAt: &finishTime,
		}

		if err != nil {
			status.Data = []entities.AudioSegment{
				{
					Start:   0,
					End:     0,
					Speaker: "ERROR",
					Text:    fmt.Sprintf("%v\n%s", err, string(out)),
				},
			}
		} else {
			// Парсим JSON, который вернул Python
			var segments []entities.AudioSegment
			if err := json.Unmarshal(out, &segments); err != nil {
				segments = []entities.AudioSegment{
					{
						Start:   0,
						End:     0,
						Speaker: "ERROR",
						Text:    fmt.Sprintf("Failed to parse JSON: %v\n%s", err, string(out)),
					},
				}
			}
			status.Data = segments
		}

		// Обновляем статус в кэше
		uc.Cache.Set(pid, status)
	}(id)

	return id, nil
}

func (uc *ProcessUsecase) StartProcessWithFile(filePath string, numSpeakers int, vadThreshold float64) (uuid.UUID, error) {
	if uc.Cache.CountRunning() >= uc.MaxProcesses {
		return uuid.Nil, fmt.Errorf("max number of concurrent processes reached")
	}

	id, _ := uuid.NewV4()
	startTime := time.Now()

	uc.Cache.Set(id, &entities.ProcessStatus{
		IsRunning: true,
		StartedAt: startTime,
	})

	go func(pid uuid.UUID) {
		defer os.Remove(filePath)

		file, err := os.Open(filePath)
		if err != nil {
			uc.Cache.Set(pid, &entities.ProcessStatus{
				IsRunning: false,
				FileName:  filepath.Base(filePath),
				StartedAt: startTime,
				Data: []entities.AudioSegment{
					{Start: 0, End: 0, Speaker: "ERROR", Text: "Failed to open file: " + err.Error()},
				},
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
			uc.Cache.Set(pid, &entities.ProcessStatus{
				IsRunning: false,
				FileName:  filepath.Base(filePath),
				StartedAt: startTime,
				Data: []entities.AudioSegment{
					{Start: 0, End: 0, Speaker: "ERROR", Text: "Failed to create request: " + err.Error()},
				},
			})
			return
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := uc.Client.Do(req)

		finishTime := time.Now()
		status := &entities.ProcessStatus{
			IsRunning:  false,
			FileName:   filepath.Base(filePath),
			StartedAt:  startTime,
			FinishedAt: &finishTime,
		}

		if err != nil {
			status.Data = []entities.AudioSegment{
				{Start: 0, End: 0, Speaker: "ERROR", Text: "Request failed: " + err.Error()},
			}
			uc.Cache.Set(pid, status)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			status.Data = []entities.AudioSegment{
				{Start: 0, End: 0, Speaker: "ERROR", Text: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))},
			}
			uc.Cache.Set(pid, status)
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
			status.Data = parsed.Results.Result
		}

		uc.Cache.Set(pid, status)
	}(id)

	return id, nil
}

func (uc *ProcessUsecase) GetStatus(id uuid.UUID) (*entities.ProcessStatus, bool) {
	return uc.Cache.Get(id)
}

func (uc *ProcessUsecase) GetAllProcessIDs() []uuid.UUID {
	return uc.Cache.GetAllProcessIDs()
}

func (uc *ProcessUsecase) WaitForCompletion(id uuid.UUID) *entities.ProcessStatus {
	// блокируемся, пока процесс не завершится
	for {
		status, exists := uc.Cache.Get(id)
		if !exists {
			return nil
		}
		if !status.IsRunning {
			return status
		}
		time.Sleep(10 * time.Second)
	}
}

func (uc *ProcessUsecase) SaveAIResult(id uuid.UUID, result []entities.AIResult) {
	if status, exists := uc.Cache.Get(id); exists {
		status.AIResult = result
	}
}
