package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"

	"git.fedesito.me/fedes1to/eserve/cmd/epull/clientConfig"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
	"git.fedesito.me/fedes1to/eserve/internal/urls"
)

// done and cancelled are clean stops, error is a failure
func readJobStream(body io.Reader) error {
	return protocol.EachSSEEvent(body, func(event protocol.StreamEvent) bool {
		terminal, err := handleStreamEvent(event.Type, event.Message)
		return err != nil || terminal
	})
}

func handleStreamEvent(eventType, message string) (bool, error) {
	fmt.Println(protocol.Colorize(protocol.StreamEvent{Type: eventType, Message: message}))
	switch eventType {
	case "done", "cancelled":
		return true, nil // clean stop
	case "error":
		return true, fmt.Errorf("racc's job ended with an error: %s", message)
	}
	return false, nil
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
