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
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gofrs/uuid"
)

type ThemeUsecase struct {
	repo         interfaces.ProccesRepository
	Cache        *cache.ProcessManager
	Client       *http.Client
	PythonAPI    string
	MaxProcesses int
}

func NewThemeUsecase(repo interfaces.ProccesRepository, c *cache.ProcessManager, cfg *config.Config) interfaces.ThemeUsecase {
	maxProc, _ := strconv.Atoi(cfg.Server.MaxProcesses)
	return &ThemeUsecase{
		repo:         repo,
		Cache:        c,
		Client:       &http.Client{},
		PythonAPI:    cfg.Server.PythonAPIURL,
		MaxProcesses: maxProc,
	}
}

func (uc *ThemeUsecase) StartDetailProcessWithFile(filePath string, numSpeakers int, vadThreshold float64) (uuid.UUID, error) {
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
