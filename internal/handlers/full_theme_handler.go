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
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// ProcessAllFilesWithSpeakerDiarizationAndAI обрабатывает ВСЕ файлы из ./miko_downloads:
// 1. Распознаёт по спикерам (V1),
// 2. Отправляет результат в AI для тематического анализа.
func (h *Handler) ProcessAllFilesWithSpeakerDiarizationAndAI(c *gin.Context) {
	downloadDir := "./miko_downloads"

	if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no downloaded files found"})
		return
	}
	speakersStr := c.DefaultQuery("speakers", "2")
	accuracyStr := c.DefaultQuery("accuracy", "0.5")

	numSpeakers, err := strconv.Atoi(speakersStr)
	if err != nil || numSpeakers < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'speakers' parameter"})
		return
	}

	vadThreshold, err := strconv.ParseFloat(accuracyStr, 64)
	if err != nil || vadThreshold < 0 || vadThreshold > 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'accuracy' parameter"})
		return
	}

	// Собираем аудиофайлы
	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read download directory"})
		return
	}

	var filePaths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".mp3" || ext == ".wav" {
			filePaths = append(filePaths, filepath.Join(downloadDir, entry.Name()))
		}
	}

	if len(filePaths) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "no audio files to process"})
		return
	}

	// Запускаем фоновую обработку
	go h.processAllFilesWithSpeakerDiarizationAndAIInParallel(filePaths, numSpeakers, vadThreshold)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Speaker diarization + AI analysis started for all files",
		"file_count": len(filePaths),
	})
}

func (h *Handler) processAllFilesWithSpeakerDiarizationAndAIInParallel(filePaths []string, numSpeakers int, vadThreshold float64) {
	maxWorkers := 1
	jobs := make(chan string, len(filePaths))
	var wg sync.WaitGroup

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range jobs {
				h.processSingleFileWithSpeakerDiarizationAndAI(filePath, numSpeakers, vadThreshold)
			}
		}()
	}
	//Если хочешь чтобы парсились все сразу закоментить этот блок
	limit := maxWorkers
	if len(filePaths) < limit {
		limit = len(filePaths)
	}
	for i := 0; i < limit; i++ {
		jobs <- filePaths[i]
	}

	// И раскоменти этот
	// for _, path := range filePaths {
	// 	jobs <- path
	// }
	close(jobs)

	wg.Wait()
	log.Printf("Completed speaker diarization + AI analysis for %d files", len(filePaths))
}

func (h *Handler) processSingleFileWithSpeakerDiarizationAndAI(filePath string, numSpeakers int, vadThreshold float64) {
	log.Printf(" Starting V1 + AI analysis for: %s", filepath.Base(filePath))

	// 1. Запускаем V1 (по спикерам)
	v1ProcID, err := h.usecase.StartDetailProcessWithFile(filePath, numSpeakers, vadThreshold)
	if err != nil {
		log.Printf("Failed to start V1 for %s: %v", filePath, err)
		return
	}

	// 2. Ждём завершения
	status := h.usecase.WaitForCompletion(v1ProcID)
	if status == nil {
		log.Printf("No status for V1 process %s", v1ProcID)
		return
	}

	v1, ok := status.Data.(*entities.ProcessStatusV1)
	if !ok || len(v1.Data) == 0 {
		log.Printf(" Invalid V1 result for %s", v1ProcID)
		return
	}

	// 3. Отправляем в AI
	text := service.FormatSegments(v1.Data)
	prompt := fmt.Sprintf(service.AnalysisPrompt, text)

	resp, err := service.ThemeRecognitionAI(
		"http://192.168.30.230:81/v1/chat/completions",
		"gpustack_ad0351498a61db96_fcad25d521f3f46e42d590e09d7d499e",
		prompt,
	)
	if err != nil {
		log.Printf(" AI error for %s: %v", v1ProcID, err)
		return
	}

	// 4. Парсим и сохраняем
	var aiResult []entities.AIResult
	if err := json.Unmarshal([]byte(resp), &aiResult); err != nil {
		log.Printf(" Parse AI error for %s: %v", v1ProcID, err)
		return
	}
	h.usecase.SaveDealAnalysisResult(v1ProcID, v1.AIResult)
	// Сохраняем в кэш (и/или БД, если нужно)
	// v1.AIResult = aiResult
	// h.usecase.Cache.Set(v1ProcID, &entities.ProcessStatus{Type: "v1", Data: v1})

	log.Printf("✅ Completed V1 + AI for %s → process %s", filepath.Base(filePath), v1ProcID)
}
