package handlers

import (
	"GoRoutine/internal/domain/entities"
	"GoRoutine/internal/service"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
)

// Распозноваине всех токсичных файлов (Парсинг по спикерам и отправка в нейронку)
func (h *Handler) ProcessToxicFilesWithFullAnalysis(c *gin.Context) {
	// 1. Получаем все токсичные процессы из БД
	processes, err := h.usecase.GetHighToxicityProcessesFromDB()
	if err != nil {
		log.Printf("❌ Failed to fetch toxic processes: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB error"})
		return
	}

	if len(processes) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "no toxic files found"})
		return
	}

	// 2. Собираем пути к файлам
	var filePaths []string
	dirs := []string{"./miko_downloads", "./tmp_uploads"}

	for _, proc := range processes {
		found := false
		for _, dir := range dirs {
			candidate := filepath.Join(dir, proc.FileName)
			if _, err := os.Stat(candidate); err == nil {
				filePaths = append(filePaths, candidate)
				found = true
				break
			}
		}
		if !found {
			log.Printf("⚠️ File not found: %s", proc.FileName)
		}
	}

	if len(filePaths) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "no toxic files with existing audio found"})
		return
	}

	// 3. Запускаем фоновую обработку
	go h.processToxicFilesInParallel(filePaths, processes)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Full analysis pipeline started for toxic files",
		"file_count": len(filePaths),
	})
}

func (h *Handler) processToxicFilesInParallel(filePaths []string, processes []entities.ProcessWithFile) {
	maxWorkers := 5 // меньше, чем для токсичности, т.к. тяжелее
	jobs := make(chan struct {
		Path string
		ID   uuid.UUID
	}, len(filePaths))
	var wg sync.WaitGroup

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				h.processToxicFileWithFullAnalysis(job.Path, job.ID)
			}
		}()
	}

	// Сопоставляем файлы с ID (упрощённо — по индексу)
	for i, path := range filePaths {
		if i < len(processes) {
			jobs <- struct {
				Path string
				ID   uuid.UUID
			}{Path: path, ID: processes[i].ID}
		}
	}
	close(jobs)

	wg.Wait()
	log.Printf("✅ Completed full analysis for %d toxic files", len(filePaths))
}

// Обрабатывает один токсичный файл: V1 → AI тематика
func (h *Handler) processToxicFileWithFullAnalysis(filePath string, originalProcID uuid.UUID) {
	log.Printf("Starting full analysis for toxic file: %s (original proc: %s)", filepath.Base(filePath), originalProcID)

	// 1. Запускаем V1-обработку (по спикерам)
	v1ProcID, err := h.usecase.StartDetailProcessWithFile(filePath, 2, 0.5)
	if err != nil {
		log.Printf("❌ Failed to start speaker diarization for %s: %v", originalProcID, err)
		return
	}

	// 2. Ждём завершения V1
	status := h.usecase.WaitForCompletion(v1ProcID)
	if status == nil {
		log.Printf("❌ No status for V1 process %s", v1ProcID)
		return
	}

	v1, ok := status.Data.(*entities.ProcessStatusV1)
	if !ok {
		log.Printf("❌ Unexpected type for V1 process %s", v1ProcID)
		return
	}

	if len(v1.Data) == 0 {
		log.Printf("❌ No segments for V1 process %s", v1ProcID)
		return
	}

	// 3. Формируем текст и отправляем в AI
	text := service.FormatSegments(v1.Data)
	prompt := fmt.Sprintf(service.AnalysisPrompt, text)

	resp, err := service.ThemeRecognitionAI(
		"http://192.168.30.230:81/v1/chat/completions",
		"gpustack_ad0351498a61db96_fcad25d521f3f46e42d590e09d7d499e",
		prompt,
	)
	if err != nil {
		log.Printf("❌ AI error for V1 process %s: %v", v1ProcID, err)
		return
	}

	// 4. Парсим результат
	var aiResult []entities.AIResult
	if err := json.Unmarshal([]byte(resp), &aiResult); err != nil {
		log.Printf("❌ Failed to parse AI response for %s: %v (raw: %s)", v1ProcID, err, resp)
		return
	}

	// 5. Сохраняем в V1-процесс
	h.usecase.SaveDealAnalysisResult(originalProcID, aiResult)

	log.Printf("✅ Full analysis completed for toxic file %s → V1: %s", originalProcID, v1ProcID)
}
