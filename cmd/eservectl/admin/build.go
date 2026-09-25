package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"git.fedesito.me/fedes1to/eserve/internal/protocol"
	"git.fedesito.me/fedes1to/eserve/internal/urls"
)

func PostStartBuild(flavor string, packages []string) (string, error) {
	payload := protocol.BuildRequest{Flavor: flavor, Packages: packages}
	body, _ := json.Marshal(payload)
	response, err := adminClient.Post(urls.SocketURL+urls.BuildStartSuburl, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode != 200 {
		return "", fmt.Errorf("couldn't start build, code %v, body:\n%v", response.StatusCode, string(bodyBytes))
	}

	var buildResponse protocol.BuildResponse
	if err := json.Unmarshal(bodyBytes, &buildResponse); err != nil || buildResponse.JobID == "" {
		return "", fmt.Errorf("couldn't start build, code %v, body:\n%v", response.StatusCode, string(bodyBytes))
	}
	return buildResponse.JobID, nil
}
