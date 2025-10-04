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

type ToxicUsecase struct {
	repo         interfaces.ProccesRepository
	Cache        *cache.ProcessManager
	Client       *http.Client
	PythonAPI    string
	MaxProcesses int
}

func NewToxicUsecase(repo interfaces.ProccesRepository, c *cache.ProcessManager, cfg *config.Config) interfaces.ToxicUsecase {
	maxProc, _ := strconv.Atoi(cfg.Server.MaxProcesses)
	return &ToxicUsecase{
		repo:         repo,
		Cache:        c,
		Client:       &http.Client{},
		PythonAPI:    cfg.Server.PythonAPIURL,
		MaxProcesses: maxProc,
	}
}

func (uc *ToxicUsecase) StartProcessWithFileAI(filePath string, numSpeakers int, vadThreshold float64) (uuid.UUID, error) {
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
		if err := uc.repo.SaveToxicProcess(pid, v2Data); err != nil {
			log.Printf("Failed to save process to DB: %v", err)
		} else {
			log.Printf("Process %s saved to DB", pid)
		}
	}(id)

	return id, nil
}
