package protocol

import (
	"bufio"
	"io"
	"strings"
)

func EachSSEEvent(r io.Reader, handle func(event StreamEvent) bool) error {
	scanner := bufio.NewScanner(r)
	// emerge's output lines can be big, let the scanner breathe
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	var eventType string
	var dataLines []string
	flush := func() bool {
		if eventType == "" {
			return false
		}
		event := StreamEvent{Type: eventType, Message: strings.Join(dataLines, "\n")}
		eventType, dataLines = "", nil
		return handle(event)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if flush() {
				return nil
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(line[len("event:"):])
		} else if strings.HasPrefix(line, "data:") {
			value := line[len("data:"):]
			if strings.HasPrefix(value, " ") {
				value = value[1:]
			}
			dataLines = append(dataLines, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil // an unterminated event at EOF is fine, the job ended
}
