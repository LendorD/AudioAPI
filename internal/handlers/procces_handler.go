package handlers

import (
	"GoRoutine/internal/domain/entities"
	"GoRoutine/internal/service"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

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
	id := h.usecase.StartProcessWithFile(filePath, numSpeakers, vadThreshold)
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func (h *Handler) GetAllProcessIDs(c *gin.Context) {
	ids := h.usecase.GetAllProcessIDs()
	c.JSON(http.StatusOK, gin.H{"IDs": ids})
}

func (h *Handler) ProcessAI(c *gin.Context) {
	procIDStr := c.Param("proc_id")
	procID, err := uuid.FromString(procIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid process id"})
		return
	}

	// Получаем статус через метод интерфейса
	status, exists := h.usecase.GetStatus(procID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "process not found"})
		return
	}

	// Формируем текст
	text := service.FormatSegments(status.Data)

	prompt := fmt.Sprintf(
		`Вот текст разговора между участниками:\n%s
	Проанализируй разговор и верни массив JSON объектов, по одному на каждую тему обсуждения.
	Каждый объект должен иметь поля:
	- theme: о чём была тема (строка)
	- deal: тип сделки (строка)
	- deal_description: кратко о деталях сделки
	- complete_deal: true/false, состоялась ли сделка
	- deal_price: число, если есть, иначе 0
	Ответи строго JSON массивом, без лишнего текста.`,
		text,
	)

	// отрпавляем в нейронку
	resp, err := service.SendToAI(
		"http://192.168.30.230:81/v1/chat/completions",
		"gpustack_ad0351498a61db96_fcad25d521f3f46e42d590e09d7d499e",
		prompt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// cleanJSON := service.ExtractJSON(resp)

	var aiResult []entities.AIResult
	if err := json.Unmarshal([]byte(resp), &aiResult); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse AI response", "raw": resp})
		return
	}

	// // Сохраняем в статус
	// status.DataFromAI = resp
	// // Если нужно обновить кэш, можно через метод StartProcess/SetStatus или добавить SetStatus
	// h.usecase.(*usecases.ProcessUsecase).Cache.Set(procID, status) // только если тип точно ProcessUsecase
	// c.JSON(http.StatusOK, aiResult)

	c.JSON(http.StatusOK, gin.H{"result": aiResult})
}
