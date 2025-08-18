package entities

import "time"

type ProcessStatus struct {
	IsRunning  bool           `json:"isRunning"`
	FileName   string         `json:"fileName,omitempty"`
	Data       []AudioSegment `json:"data,omitempty"`
	StartedAt  time.Time      `json:"startedAt"`
	FinishedAt *time.Time     `json:"finishedAt,omitempty"`
	AIResult   []AIResult     `json:"aiResult,omitempty"`
}
