package admin

import (
	"fmt"
	"io"
)

func PostCreateToken() (string, error) {
	response, err := adminClient.Post("http://unix/admin/v1/create_token", "", nil)
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
