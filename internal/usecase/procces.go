package usecase

import (
	"GoRoutine/internal/cache"
	"GoRoutine/internal/domain/entities"
	"GoRoutine/internal/interfaces"
	"fmt"
	"os"
	"os/exec"
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
		cmd.Env = append(os.Environ(),
			"PATH=C:\\Users\\dlucenko\\Desktop\\AudioAPI\\AudioAPI\\venv\\Scripts;"+os.Getenv("PATH"))

		// Собираем stdout и stderr
		out, err := cmd.CombinedOutput()

		finishTime := time.Now()
		status := &entities.ProcessStatus{
			IsRunning:  false,
			StartedAt:  startTime,
			FinishedAt: &finishTime,
			Data:       string(out),
		}

		if err != nil {
			status.Data = fmt.Sprintf("[ERROR]: %v\n%s", err, out)
		}

		// Обновляем статус в кэше
		uc.Cache.Set(pid, status)
	}(id)

	return id
}

func (uc *ProcessUsecase) StartProcessWithFile(filePath string) uuid.UUID {
	id, _ := uuid.NewV4()
	startTime := time.Now()

	uc.Cache.Set(id, &entities.ProcessStatus{
		IsRunning: true,
		StartedAt: startTime,
	})

	go func(pid uuid.UUID) {
		cmd := exec.Command("python", "./python-scripts/script.py", filePath)
		out, err := cmd.CombinedOutput()

		os.Remove(filePath)

		finishTime := time.Now()
		status := &entities.ProcessStatus{
			IsRunning:  false,
			StartedAt:  startTime,
			FinishedAt: &finishTime,
			Data:       string(out),
		}

		if err != nil {
			status.Data = fmt.Sprintf("[ERROR]: %v\n%s", err, out)
		}

		uc.Cache.Set(pid, status)
	}(id)

	return id
}

func (uc *ProcessUsecase) GetStatus(id uuid.UUID) (*entities.ProcessStatus, bool) {
	return uc.Cache.Get(id)
}
