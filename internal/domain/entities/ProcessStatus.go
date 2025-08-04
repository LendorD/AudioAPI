package entities

import "time"

type ProcessStatus struct {
	IsRunning  bool
	Data       any
	StartedAt  time.Time
	FinishedAt *time.Time // nil, если ещё не завершён
}
