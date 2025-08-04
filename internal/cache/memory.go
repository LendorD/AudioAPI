package cache

import (
	"GoRoutine/internal/domain/entities"
	"github.com/gofrs/uuid"
	"sync"
	"time"
)

type ProcessManager struct {
	mu    sync.RWMutex
	store map[uuid.UUID]*entities.ProcessStatus
}

func NewProcessCache() *ProcessManager {
	return &ProcessManager{
		store: make(map[uuid.UUID]*entities.ProcessStatus),
	}
}

func (c *ProcessManager) Set(id uuid.UUID, status *entities.ProcessStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[id] = status
}

func (c *ProcessManager) Get(id uuid.UUID) (*entities.ProcessStatus, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	status, ok := c.store[id]
	return status, ok
}

// Очистка записей старше 24 часов
func (pm *ProcessManager) CleanupOldProcesses() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	for id, status := range pm.store {
		if status.FinishedAt != nil && now.Sub(*status.FinishedAt) > 24*time.Hour {
			delete(pm.store, id)
		}
	}
}
