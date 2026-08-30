package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"git.fedesito.me/fedes1to/eserve/internal/protocol"
	"git.fedesito.me/fedes1to/eserve/internal/urls"
)

func PostListJobs() (protocol.JobListResponse, error) {
	response, err := adminClient.Post(urls.SocketURL+urls.JobsListSuburl, "", nil)
	if err != nil {
		return protocol.JobListResponse{}, err
	}
	defer response.Body.Close()

	var list protocol.JobListResponse
	if err := json.NewDecoder(response.Body).Decode(&list); err != nil {
		return protocol.JobListResponse{}, err
	}
	if response.StatusCode != 200 {
		return protocol.JobListResponse{}, fmt.Errorf("couldn't list jobs, code %v", response.StatusCode)
	}
	return list, nil
}

func PostCancelJob(id string) error {
	payload := protocol.JobRequest{JobID: id}
	body, _ := json.Marshal(payload)
	response, err := adminClient.Post(urls.SocketURL+urls.AdminJobsCancelSuburl, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != 200 {
		return fmt.Errorf("couldn't cancel job, code %v, body:\n%v", response.StatusCode, string(bodyBytes))
	}
	return nil
}

func PostJobStream(id string) (terminal string, success bool, err error) {
	payload := protocol.JobRequest{JobID: id}
	body, _ := json.Marshal(payload)
	response, err := adminClient.Post(urls.SocketURL+urls.AdminJobsStreamSuburl, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", false, err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(response.Body)
		return "", false, fmt.Errorf("couldn't stream job, code %v, body:\n%v", response.StatusCode, string(bodyBytes))
	}

	finished := false
	err = protocol.EachSSEEvent(response.Body, func(event protocol.StreamEvent) bool {
		fmt.Fprint(os.Stdout, protocol.Colorize(event))
		if !strings.HasSuffix(event.Message, "\n") {
			fmt.Println()
		}
		switch event.Type {
		case "done":
			terminal, success, finished = "done", true, true
		case "error":
			terminal, success, finished = "error", false, true
		case "cancelled":
			terminal, success, finished = "cancelled", true, true
		}
		return finished
	})
	if err != nil {
		return "", false, err
	}
	if !finished {
		return "", false, fmt.Errorf("stream ended without a terminal event")
	}
	return terminal, success, nil
}
