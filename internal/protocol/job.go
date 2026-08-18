package protocol

type JobRequest struct {
	JobID string `json:"job_id"`
}

type StreamEvent struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
}
