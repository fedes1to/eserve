package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"git.fedesito.me/fedes1to/eserve/internal/protocol"
	"git.fedesito.me/fedes1to/eserve/internal/urls"
)

func PostCreateToken() (string, error) {
	response, err := adminClient.Post(urls.SocketURL+urls.CreateTokenSuburl, "", nil)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	bodyString := string(bodyBytes)
	if response.StatusCode != 200 {
		return "", fmt.Errorf("couldn't create token, code %v, body:\n%v", response.StatusCode, bodyString)
	}

	return bodyString, nil
}

func PostListTokens() ([]protocol.TokenInfo, error) {
	response, err := adminClient.Post(urls.SocketURL+urls.TokensListSuburl, "", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var list protocol.TokenListResponse
	if err := json.NewDecoder(response.Body).Decode(&list); err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("couldn't list tokens, code %v", response.StatusCode)
	}
	return list.Tokens, nil
}

func PostDeleteToken(token string) error {
	payload := protocol.DeleteTokenRequest{Token: token}
	body, _ := json.Marshal(payload)
	response, err := adminClient.Post(urls.SocketURL+urls.TokenDeleteSuburl, "application/json", bytes.NewReader(body))
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
		return fmt.Errorf("couldn't delete token, code %v, body:\n%v", response.StatusCode, bodyString)
	}

	return nil
}
