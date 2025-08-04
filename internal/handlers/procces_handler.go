package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"net/http"
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
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": status})
}
