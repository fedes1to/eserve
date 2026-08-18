package admin

import (
	"context"
	"net"
	"net/http"
	"time"

	"git.fedesito.me/fedes1to/eserve/internal/urls"
)

var adminClient *http.Client = initAdminClient()

func TryConnect() error {
	dial, err := net.DialTimeout("unix", urls.SocketPath, time.Second)
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
				return (&net.Dialer{}).DialContext(ctx, "unix", urls.SocketPath)
			},
		},
	}
}
