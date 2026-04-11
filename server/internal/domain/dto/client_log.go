package dto

// ClientLogReq 客户端日志上报
type ClientLogReq struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Error     string                 `json:"error,omitempty"`
	StackTrace string                `json:"stackTrace,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Timestamp string                 `json:"timestamp"`
}
