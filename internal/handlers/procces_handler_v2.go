package handlers

import (
	"GoRoutine/internal/domain/entities"
	"GoRoutine/internal/service"
	"context"
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
	// 1. Получаем токен с внешнего сервиса
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

	// Предполагаем, что токен приходит в теле как plain text (а не JSON)
	// Если приходит JSON — см. примечание ниже
	token := strings.TrimSpace(string(body))
	if token == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "received empty token"})
		return
	}

	// 2. Передаём токен в вашу функцию
	records, err := service.MikoGetFilesName(token)
	if err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"records": records})
}

func (h *Handler) ProcessAllDownloadedFiles(c *gin.Context) {
	downloadDir := "./miko_downloads"

	// Проверяем существование директории
	if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no downloaded files found"})
		return
	}

	// Получаем параметры из query
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

	// Читаем файлы
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
		// Опционально: фильтр по расширению
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".mp3" && ext != ".wav" && ext != ".ogg" && ext != ".flac" {
			continue
		}
		filePaths = append(filePaths, filepath.Join(downloadDir, entry.Name()))
	}

	if len(filePaths) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message":           "no audio files found in download directory",
			"started_processes": []string{},
		})
		return
	}

	// ⚙️ Настройки параллелизма
	maxWorkers := 5 // можно вынести в конфиг или параметр
	jobs := make(chan string, len(filePaths))
	results := make(chan struct {
		ID    uuid.UUID
		Error string
	}, len(filePaths))

	var wg sync.WaitGroup

	// Запускаем воркеров
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range jobs {
				id, err := h.usecase.StartProcessWithFileAI(filePath, numSpeakers, vadThreshold)
				if err != nil {
					results <- struct {
						ID    uuid.UUID
						Error string
					}{ID: uuid.Nil, Error: fmt.Sprintf("file %s: %v", filepath.Base(filePath), err)}
				} else {
					results <- struct {
						ID    uuid.UUID
						Error string
					}{ID: id, Error: ""}
				}
			}
		}()
	}

	// Отправляем задачи
	go func() {
		defer close(jobs)
		for _, path := range filePaths {
			jobs <- path
		}
	}()

	// Ждём завершения воркеров
	go func() {
		wg.Wait()
		close(results)
	}()

	// Собираем результаты с таймаутом (на случай зависания)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var processIDs []uuid.UUID
	var errors []string

	for {
		select {
		case res, ok := <-results:
			if !ok {
				// Канал закрыт — все результаты получены
				c.JSON(http.StatusOK, gin.H{
					"started_processes": processIDs,
					"errors":            errors,
				})
				return
			}
			if res.Error != "" {
				errors = append(errors, res.Error)
			} else {
				processIDs = append(processIDs, res.ID)
			}
		case <-ctx.Done():
			// Таймаут — возвращаем то, что успели
			c.JSON(http.StatusPartialContent, gin.H{
				"started_processes": processIDs,
				"errors":            append(errors, "timeout while waiting for all processes to start"),
			})
			return
		}
	}
}

func (h *Handler) StartWithFileAI(c *gin.Context) {
	// Получаем файл
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	// кол-во говорящих
	speakersStr := c.DefaultQuery("speakers", "2")
	// порог детекции речи (0-1)
	vadStr := c.DefaultQuery("accuracy", "0.5")

	numSpeakers, err := strconv.Atoi(speakersStr)
	if err != nil || numSpeakers < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid speakers parameter"})
		return
	}

	vadThreshold, err := strconv.ParseFloat(vadStr, 64)
	if err != nil || vadThreshold < 0 || vadThreshold > 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid accuracy parameter"})
		return
	}

	// Создаём временную папку, если нет
	tmpDir := "./tmp_uploads"
	os.MkdirAll(tmpDir, os.ModePerm)

	// Полный путь к файлу
	filePath := filepath.Join(tmpDir, file.Filename)

	// Сохраняем загруженный файл
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	// Запускаем процесс с файлом
	id, err := h.usecase.StartProcessWithFileAI(filePath, numSpeakers, vadThreshold)
	if err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func (h *Handler) StartToxicityAnalysisPipeline(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	speakersStr := c.DefaultQuery("speakers", "2")
	vadStr := c.DefaultQuery("accuracy", "0.5")

	numSpeakers, _ := strconv.Atoi(speakersStr)
	vadThreshold, _ := strconv.ParseFloat(vadStr, 64)

	tmpDir := "./tmp_uploads"
	os.MkdirAll(tmpDir, os.ModePerm)
	filePath := filepath.Join(tmpDir, file.Filename)
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	// Запускаем процесс обработки (v2 — получаем DataRaw)
	procID, err := h.usecase.StartProcessWithFileAI(filePath, numSpeakers, vadThreshold)
	if err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}

	// Фоновая горутина: ждём завершения → запускаем AI-анализ токсичности
	go func() {
		log.Printf("Process %s start", procID)
		// Блокирующий метод ожидания завершения
		status := h.usecase.WaitForCompletion(procID)
		if status == nil {
			return
		}

		// Получаем ProcessStatusV2 из Data
		v2, ok := status.Data.(*entities.ProcessStatusV2)
		if !ok {
			log.Printf("Process %s is not V2 type", procID)
			return
		}

		// Проверяем, что DataRaw и Content существуют
		if v2.DataRaw == nil || v2.DataRaw.Content == "" {
			log.Printf("No content to analyze for process %s", procID)
			return
		}

		// Формируем промт для анализа токсичности
		prompt := fmt.Sprintf(service.ToxicityAnalysisPrompt, v2.DataRaw.Content)
		log.Printf("Process %s send AI", procID)
		// Отправляем в AI
		resp, err := service.ThemeRecognitionAI(
			"http://192.168.30.230:81/v1/chat/completions",
			"gpustack_ad0351498a61db96_fcad25d521f3f46e42d590e09d7d499e",
			prompt,
		)
		if err != nil {
			log.Println("Error from AI: ", err.Error())
			return
		}

		// Парсим ответ как ToxicityAnalysis
		var toxicityResult entities.ToxicityAnalysis
		if err := json.Unmarshal([]byte(resp), &toxicityResult); err != nil {
			log.Printf("Failed to parse AI response for %s: %v", procID, err)
			return
		}
		log.Printf("Process %s save AI result", procID)

		// Сохраняем результат через специализированный метод
		if err := h.usecase.SaveToxicityAnalysisResult(procID, toxicityResult); err != nil {
			log.Printf("Failed to save toxicity result for %s: %v", procID, err)
			return
		}

		log.Printf("Toxicity analysis completed for process %s", procID)
	}()

	// Клиенту сразу возвращаем ID
	c.JSON(http.StatusOK, gin.H{"id": procID})
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
