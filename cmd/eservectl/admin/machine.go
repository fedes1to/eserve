package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"git.fedesito.me/fedes1to/eserve/internal/protocol"
	"git.fedesito.me/fedes1to/eserve/internal/urls"
)

func PostRevokeMachine(cn string) error {
	payload := protocol.RevokeRequest{CN: cn}
	body, _ := json.Marshal(payload)
	response, err := adminClient.Post(urls.SocketURL+urls.RevokeMachineSuburl, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	bodyString := string(bodyBytes)
	if response.StatusCode != 200 || bodyString != "ok" {
		return fmt.Errorf("couldn't revoke machine, code %v, body:\n%v", response.StatusCode, bodyString)
	}

	return nil
}
