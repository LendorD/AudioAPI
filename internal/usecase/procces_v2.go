package usecase

import (
	"GoRoutine/internal/cache"
	"GoRoutine/internal/config"
	"GoRoutine/internal/domain/entities"
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
	Cache        *cache.ProcessManager
	Client       *http.Client
	PythonAPI    string
	MaxProcesses int
}

func NewProcessUsecaseAI(c *cache.ProcessManager, cfg *config.Config) *ProcessUsecaseAI {
	maxProc, _ := strconv.Atoi(cfg.Server.MaxProcesses)
	return &ProcessUsecaseAI{
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

		// Указываем Content-Type явно как в curl/Postman
		partHeaders := textproto.MIMEHeader{}
		partHeaders.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "file", filepath.Base(filePath)))
		partHeaders.Set("Content-Type", "audio/mpeg")

		part, err := writer.CreatePart(partHeaders)
		if err != nil {
			uc.Cache.Set(pid, &entities.ProcessStatus{
				IsRunning: false,
				FileName:  filepath.Base(filePath),
				StartedAt: startTime,
			})
			return
		}

		_, err = io.Copy(part, file)
		if err != nil {
			uc.Cache.Set(pid, &entities.ProcessStatus{
				IsRunning: false,
				FileName:  filepath.Base(filePath),
				StartedAt: startTime,
				Data: []entities.AudioSegment{
					{Start: 0, End: 0, Speaker: "ERROR", Text: "Failed to copy file to request body: " + err.Error()},
				},
			})
			return
		}

		writer.Close()

		req, err := http.NewRequest("POST", "http://192.168.30.230:3001/api/v1/files/", &buf)
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
		req.Header.Set("Authorization", "Bearer "+"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjkzMDEzZTUyLTkzMDMtNDFjOC04ZjAyLTZkYjM5NjMwMDY5MiJ9.h9U4nUMBX0RifEwIp8lf40G20Na66kBmCJy1Au25UaY")

		resp, err := uc.Client.Do(req)

		finishTime := time.Now()
		status := &entities.ProcessStatus{
			IsRunning:  false,
			FileName:   filepath.Base(filePath),
			StartedAt:  startTime,
			FinishedAt: &finishTime,
		}

		if err != nil {
			status.DataRaw.Content = "Request failed: " + err.Error()
			uc.Cache.Set(pid, status)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			status.DataRaw.Content = "HTTP: " + fmt.Sprint(resp.StatusCode)
			uc.Cache.Set(pid, status)
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
			status.DataRaw = &entities.AIContent{
				ID:      "",
				Content: fmt.Sprintf("Failed to parse JSON: %v\n%s", err, string(body)),
			}
		} else {
			status.DataRaw = &entities.AIContent{
				ID:      parsed.ID,
				Content: parsed.Data.Content,
			}
		}

		uc.Cache.Set(pid, status)
	}(id)

	return id, nil
}
