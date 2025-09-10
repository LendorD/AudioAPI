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

	"github.com/gin-gonic/gin"
)

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
