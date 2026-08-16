package admin

import (
	"context"
	"net"
	"net/http"
	"time"
)

var socketPath string = "/run/eserved.sock"
var adminClient *http.Client = initAdminClient()

func TryConnect() error {
	dial, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		return err
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
