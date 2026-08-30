package protocol

type JobRequest struct {
	JobID string `json:"job_id"`
}

type StreamEvent struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
}

// ANSI colors for display; epull colorizes events itself, the wire carries none
const (
	Reset = "\x1b[0m"
	Cyan  = "\x1b[36m"
	Green = "\x1b[32m"
	Red   = "\x1b[31m"
	Bold  = "\x1b[1m"
)

func Color(eventType string) string {
	switch eventType {
	case "progress":
		return Cyan
	case "done":
		return Green + Bold
	case "error", "cancelled":
		return Red
	}
	return ""
}

func Colorize(event StreamEvent) string {
	if color := Color(event.Type); color != "" {
		return color + event.Message + Reset
	}
	return event.Message
}
