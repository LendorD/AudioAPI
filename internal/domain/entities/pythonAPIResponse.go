package entities

// структура под ответ питона
type PythonAPIResponse struct {
	Status    string `json:"status"`
	Msg       string `json:"msg"`
	Timestamp string `json:"timestamp"`
	FileInfo  struct {
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		SizeBytes   int64  `json:"size_bytes"`
	} `json:"file_info"`
	ProcessingParams struct {
		Speakers int     `json:"speakers"`
		Accuracy float64 `json:"accuracy"`
	} `json:"processing_params"`
	Results struct {
		TaskID   string         `json:"task_id"`
		FilePath string         `json:"file_path"`
		ModelID  int            `json:"model_id"`
		Status   string         `json:"status"`
		Result   []AudioSegment `json:"result"`
	} `json:"results"`
	SegmentCount int `json:"segment_count"`
}
