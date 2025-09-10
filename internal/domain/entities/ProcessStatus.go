package entities

import "time"

// ProcessStatusCommon — общие поля для всех версий
type ProcessStatusCommon struct {
	IsRunning  bool       `json:"isRunning"`
	FileName   string     `json:"fileName,omitempty"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

// ProcessStatusV1 — данные для API v1
type ProcessStatusV1 struct {
	ProcessStatusCommon
	Data     []AudioSegment `json:"data,omitempty"`
	AIResult []AIResult     `json:"aiResult,omitempty"`
}

// ProcessStatusV2 — данные для API v2
type ProcessStatusV2 struct {
	ProcessStatusCommon
	DataRaw  *AIContent       `json:"dataRaw,omitempty"`
	AIResult ToxicityAnalysis `json:"aiResult,omitempty"`
}

// ProcessStatus — универсальная обертка для хранения в кэше
type ProcessStatus struct {
	Type string      `json:"type"` // "v1" или "v2"
	Data interface{} `json:"data"` // *ProcessStatusV1 или *ProcessStatusV2
}

type ToxicityAnalysis struct {
	ToxicityLevel        string          `json:"toxicity_level"`
	Flags                []string        `json:"flags"`
	Summary              string          `json:"summary"`
	ProfessionalismScore int             `json:"professionalism_score"`
	RiskAssessment       string          `json:"risk_assessment"`
	Quotes               []ToxicityQuote `json:"quotes"`
}

type ToxicityQuote struct {
	Text     string `json:"text"`
	Reason   string `json:"reason"`
	Category string `json:"category"`
}

type AIContent struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

func (ps *ProcessStatus) GetIsRunning() bool {
	switch v := ps.Data.(type) {
	case *ProcessStatusV1:
		return v.IsRunning
	case *ProcessStatusV2:
		return v.IsRunning
	default:
		return false
	}
}
