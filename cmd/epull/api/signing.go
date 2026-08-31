package api

import (
	"fmt"
	"io"

	"git.fedesito.me/fedes1to/eserve/cmd/epull/storage"
	"git.fedesito.me/fedes1to/eserve/internal/urls"
)

func SetupSigningKey() error {
	response, err := sendMtlsGetRaw(urls.SigningKeySuburl)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	key, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if len(key) == 0 {
		return fmt.Errorf("the server has no signing key")
	}
	return storage.SetupGpgKey(string(key))
}
