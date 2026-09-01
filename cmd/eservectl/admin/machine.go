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

func PostListMachines() ([]protocol.MachineInfo, error) {
	response, err := adminClient.Post(urls.SocketURL+urls.MachinesListSuburl, "", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var list protocol.MachineListResponse
	if err := json.NewDecoder(response.Body).Decode(&list); err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("couldn't list machines, code %v", response.StatusCode)
	}
	return list.Machines, nil
}

func PostDeleteMachine(cn string) error {
	payload := protocol.DeleteMachineRequest{CN: cn}
	body, _ := json.Marshal(payload)
	response, err := adminClient.Post(urls.SocketURL+urls.MachineDeleteSuburl, "application/json", bytes.NewReader(body))
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
		return fmt.Errorf("couldn't delete machine, code %v, body:\n%v", response.StatusCode, bodyString)
	}

	return nil
}
