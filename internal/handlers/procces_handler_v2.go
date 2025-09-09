package handlers

import (
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
