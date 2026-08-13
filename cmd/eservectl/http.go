package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

var socketPath string = "/run/eserved.sock"
var adminClient *http.Client = initAdminClient()

func tryConnect() error {
	dial, dialError := net.DialTimeout("unix", socketPath, time.Second)
	if dialError != nil {
		return dialError
	}
	dial.Close()
	return nil
}

func initAdminClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		},
	}
}

func createToken() (string, error) {
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
