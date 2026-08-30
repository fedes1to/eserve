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

	var buildResponse protocol.BuildResponse
	if err := json.NewDecoder(response.Body).Decode(&buildResponse); err != nil {
		bodyBytes, _ := io.ReadAll(response.Body)
		return "", fmt.Errorf("couldn't start build, code %v, body:\n%v", response.StatusCode, string(bodyBytes))
	}
	if response.StatusCode != 200 || buildResponse.JobID == "" {
		return "", fmt.Errorf("couldn't start build, code %v", response.StatusCode)
	}
	return buildResponse.JobID, nil
}
