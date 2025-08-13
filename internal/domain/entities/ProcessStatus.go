package entities

import "time"

type ProcessStatus struct {
	IsRunning  bool           `json:"isRunning"`
	Data       []AudioSegment `json:"data,omitempty"`
	StartedAt  time.Time      `json:"startedAt"`
	FinishedAt *time.Time     `json:"finishedAt,omitempty"`
}
