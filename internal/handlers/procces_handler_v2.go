package handlers

import (
	"GoRoutine/internal/domain/entities"
	"GoRoutine/internal/service"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
)

func (h *Handler) GetFilesName(c *gin.Context) {
	// Получаем даты из query
	fromStr := c.DefaultQuery("from", "")
	toStr := c.DefaultQuery("to", "")

	var from, to time.Time
	var err error

	if fromStr == "" || toStr == "" {
		now := time.Now()
		// Берём весь текущий день
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		to = from.Add(24 * time.Hour).Add(-time.Second)
	} else {
		// Парсим даты в формате DD-MM-YYYY
		from, err = time.Parse("02-01-2006", fromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'from' date format, use DD-MM-YYYY"})
			return
		}
		to, err = time.Parse("02-01-2006", toStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'to' date format, use DD-MM-YYYY"})
			return
		}
		// Делаем диапазон включительно: весь день "to"
		to = time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 0, to.Location())
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get("http://192.168.9.77:9105/token")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch token: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("token service returned status %d", resp.StatusCode),
		})
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read token response: " + err.Error()})
		return
	}

	token := strings.TrimSpace(string(body))
	if token == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "received empty token"})
		return
	}

	records, err := service.MikoGetFilesName(token, from, to)
	if err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"records": records})
}

func (h *Handler) GetAllProcessIDs(c *gin.Context) {
	ids := h.usecase.GetAllProcessIDs()
	c.JSON(http.StatusOK, gin.H{"IDs": ids})
}

func (h *Handler) GetStatus(c *gin.Context) {
	rawId := c.Param("proc_id")
	id, err := uuid.FromString(rawId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, ok := h.usecase.GetStatus(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "process not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": status})
}

func (h *Handler) ProcessAllWithToxicityAnalysis(c *gin.Context) {
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
		if ext != ".mp3" && ext != ".wav" {
			continue
		}
		filePaths = append(filePaths, filepath.Join(downloadDir, entry.Name()))
	}

	if len(filePaths) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "no audio files to process"})
		return
	}

	// Запускаем фоновую обработку (не блокируем ответ)
	go h.processAllFilesInParallel(filePaths, numSpeakers, vadThreshold)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Toxicity analysis pipeline started for all files",
		"file_count": len(filePaths),
	})
}

// Вспомогательная функция для фоновой обработки
func (h *Handler) processAllFilesInParallel(filePaths []string, numSpeakers int, vadThreshold float64) {
	maxWorkers := 10
	jobs := make(chan string, len(filePaths))
	var wg sync.WaitGroup

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range jobs {
				h.processSingleFileWithToxicity(filePath, numSpeakers, vadThreshold)
			}
		}()
	}

	for _, path := range filePaths {
		jobs <- path
	}
	close(jobs)

	wg.Wait()
	log.Printf("Completed toxicity analysis for %d files", len(filePaths))
}

// Обработка одного файла
func (h *Handler) processSingleFileWithToxicity(filePath string, numSpeakers int, vadThreshold float64) {
	log.Printf("Starting pipeline for: %s", filepath.Base(filePath))

	procID, err := h.usecase.StartProcessWithFileAI(filePath, numSpeakers, vadThreshold)
	if err != nil {
		log.Printf("Failed to start process for %s: %v", filePath, err)
		return
	}

	status := h.usecase.WaitForCompletion(procID)
	if status == nil {
		log.Printf("Process %s: no status", procID)
		return
	}

	v2, ok := status.Data.(*entities.ProcessStatusV2)
	if !ok {
		log.Printf("Process %s: unexpected type", procID)
		return
	}

	if v2.DataRaw == nil || v2.DataRaw.Content == "" {
		log.Printf("No content for %s", procID)
		return
	}

	prompt := fmt.Sprintf(service.ToxicityAnalysisPrompt, v2.DataRaw.Content)
	resp, err := service.ThemeRecognitionAI(
		"http://192.168.30.230:81/v1/chat/completions",
		"gpustack_ad0351498a61db96_fcad25d521f3f46e42d590e09d7d499e",
		prompt,
	)
	if err != nil {
		log.Printf("AI error for %s: %v", procID, err)
		return
	}

	var toxicityResult entities.ToxicityAnalysis
	if err := json.Unmarshal([]byte(resp), &toxicityResult); err != nil {
		log.Printf("Parse AI error for %s: %v", procID, err)
		return
	}

	if err := h.usecase.SaveToxicityAnalysisResult(procID, toxicityResult); err != nil {
		log.Printf("Save toxicity error for %s: %v", procID, err)
		return
	}

	log.Printf("✅ Toxicity analysis completed for %s", procID)
}

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

func (h *Handler) ProcessSingleFileWithToxicity(c *gin.Context) {
	// 1. Получаем файл
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	// 2. Параметры (опционально)
	speakersStr := c.DefaultQuery("speakers", "2")
	accuracyStr := c.DefaultQuery("accuracy", "0.5")

	numSpeakers, _ := strconv.Atoi(speakersStr)
	vadThreshold, _ := strconv.ParseFloat(accuracyStr, 64)

	// 3. Сохраняем файл
	tmpDir := "./tmp_uploads"
	os.MkdirAll(tmpDir, os.ModePerm)
	filePath := filepath.Join(tmpDir, file.Filename)
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	// 4. Запускаем фоновую обработку
	go func() {
		h.processSingleFileWithToxicity(filePath, numSpeakers, vadThreshold)
	}()

	// 5. Сразу отвечаем клиенту
	c.JSON(http.StatusOK, gin.H{
		"message":  "File processing started",
		"filename": file.Filename,
	})
}
