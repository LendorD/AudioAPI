package entities

import "time"

type ProcessStatus struct {
	IsRunning  bool           `json:"isRunning"`
	FileName   string         `json:"fileName,omitempty"`
	Data       []AudioSegment `json:"data,omitempty"`
	DataRaw    *AIContent     `json:"dataRaw,omitempty"`
	StartedAt  time.Time      `json:"startedAt"`
	FinishedAt *time.Time     `json:"finishedAt,omitempty"`
	AIResult   []AIResult     `json:"aiResult,omitempty"`
}

type AIContent struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}
