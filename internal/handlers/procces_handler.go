package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
)

func (h *Handler) Start(c *gin.Context) {
	id := h.usecase.StartProcess()
	c.JSON(http.StatusOK, gin.H{"id": id})
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

func (h *Handler) StartWithFile(c *gin.Context) {
	// Получаем файл
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
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
	id := h.usecase.StartProcessWithFile(filePath)
	c.JSON(http.StatusOK, gin.H{"id": id})
}
