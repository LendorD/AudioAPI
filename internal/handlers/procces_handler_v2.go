package handlers

import (
	"GoRoutine/internal/service"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
)

func (h *Handler) GetFilesName(c *gin.Context) {
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
