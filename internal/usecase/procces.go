package usecase

import (
	"GoRoutine/internal/cache"
	"GoRoutine/internal/domain/entities"
	"GoRoutine/internal/interfaces"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gofrs/uuid"
)

type ProcessUsecase struct {
	Cache *cache.ProcessManager
}

func NewProcessUsecase(c *cache.ProcessManager) interfaces.ProcessUsecase {
	return &ProcessUsecase{Cache: c}
}

func (uc *ProcessUsecase) StartProcess() uuid.UUID {
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

	return id
}

func (uc *ProcessUsecase) StartProcessWithFile(filePath string, numSpeakers int, vadThreshold float64) uuid.UUID {
	id, _ := uuid.NewV4()
	startTime := time.Now()

	uc.Cache.Set(id, &entities.ProcessStatus{
		IsRunning: true,
		StartedAt: startTime,
	})

	go func(pid uuid.UUID) {
		cmd := exec.Command(
			"python",
			"./python-scripts/script.py",
			filePath,
			fmt.Sprintf("%d", numSpeakers),
			fmt.Sprintf("%f", vadThreshold),
		)

		out, err := cmd.CombinedOutput()

		finishTime := time.Now()

		os.Remove(filePath)

		status := &entities.ProcessStatus{
			IsRunning:  false,
			FileName:   filepath.Base(filePath),
			StartedAt:  startTime,
			FinishedAt: &finishTime,
		}

		// Если ошибка — оставляем текст ошибки в Data как одно сегментное сообщение
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

		uc.Cache.Set(pid, status)
	}(id)

	return id
}

func (uc *ProcessUsecase) GetStatus(id uuid.UUID) (*entities.ProcessStatus, bool) {
	return uc.Cache.Get(id)
}

func (uc *ProcessUsecase) GetAllProcessIDs() []uuid.UUID {
	return uc.Cache.GetAllProcessIDs()
}
