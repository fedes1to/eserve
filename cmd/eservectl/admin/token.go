package admin

import (
	"fmt"
	"io"
)

func CreateToken() (string, error) {
	response, postError := adminClient.Post("http://unix/admin/v1/create_token", "", nil)
	if postError != nil {
		return "", postError
	}
	defer response.Body.Close()

	bodyBytes, ioError := io.ReadAll(response.Body)
	if ioError != nil {
		return "", ioError
	}

	bodyString := string(bodyBytes)
	if response.StatusCode != 200 {
		return "", fmt.Errorf("couldn't create token, code %v, body:\n%v", response.StatusCode, bodyString)
	}

	return bodyString, nil
}
