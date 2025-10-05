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

	log.Printf("Toxicity analysis completed for %s", procID)
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
