package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"git.fedesito.me/fedes1to/eserve/cmd/epull/clientConfig"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
	"git.fedesito.me/fedes1to/eserve/internal/urls"
)

func handleJobEvent(event protocol.StreamEvent) error {
	// display line or turn terminal event into an error
	switch event.Type {
	case "progress", "output":
		fmt.Println(event.Message)
	case "error":
		return fmt.Errorf("job failed: %s", event.Message)
	case "cancelled":
		return fmt.Errorf("job cancelled: %s", event.Message)
	}
	return nil
}

func readJobStream(body io.Reader) (err error) {
	scanner := bufio.NewScanner(body)
	eventType := ""
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case line == "":
			if eventType == "" {
				continue // stray blank line
			}

			// blank line ends the SSE event, parse and dispatch
			payload := strings.Join(dataLines, "\n")
			var event protocol.StreamEvent
			if err := json.Unmarshal([]byte(payload), &event); err != nil {
				return fmt.Errorf("couldn't decode stream event %q: %w", payload, err)
			}

			if event.Type == "done" {
				return nil
			}

			if err := handleJobEvent(event); err != nil {
				return err
			}

			eventType = ""
			dataLines = nil

		case strings.HasPrefix(line, "event:"):
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:")) // event name

		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines,
				strings.TrimSpace(strings.TrimPrefix(line, "data:"))) // JSON payload line
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return fmt.Errorf("racc job stream ended without a terminal event")
}

func openJobStream(jobID string) (response *http.Response, err error) {
	payload := protocol.JobRequest{JobID: jobID}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	response, err = mtlsClient.Post(
		clientConfig.Settings.Server+urls.JobsStreamSuburl,
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(response.Body)
		response.Body.Close()
		return nil, fmt.Errorf("racc couldn't stream job, code %v, body:\n%v",
			response.StatusCode, string(bodyBytes))
	}

	return response, nil
}

func watchInterrupts(jobID string, done <-chan struct{}) {
	// 1st ctrl+c cancels, 2nd kills epull
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	select {
	case <-signals:
		fmt.Println("\nCancellation requested...")
		// cancel async so the stream keeps printing
		go func() {
			if err := postCancelJob(jobID); err != nil {
				fmt.Fprintln(os.Stderr, "racc failed to cancel job:", err)
			}
		}()

		<-signals
		os.Exit(130)

	case <-done:
	}
}

func getStreamJob(jobID string) (err error) {
	response, err := openJobStream(jobID)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	done := make(chan struct{})
	defer close(done)

	go watchInterrupts(jobID, done)

	return readJobStream(response.Body)
}

func postCancelJob(jobID string) error {
	payload := protocol.JobRequest{JobID: jobID}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	response, err := mtlsClient.Post(
		clientConfig.Settings.Server+urls.JobsCancelSuburl,
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(response.Body)
		return fmt.Errorf("racc couldn't cancel job, code %v, body:\n%v",
			response.StatusCode, string(bodyBytes))
	}

	return nil
}
