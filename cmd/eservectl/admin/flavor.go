package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"git.fedesito.me/fedes1to/eserve/internal/urls"
)

func PostApplyFlavor(flavor string) error {
	payload := map[string]string{"flavor": flavor}
	body, _ := json.Marshal(payload)
	response, err := adminClient.Post(urls.SocketURL+urls.FlavorApplySuburl, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != 200 || string(bodyBytes) != "ok" {
		return fmt.Errorf("couldn't apply flavor config, code %v, body:\n%v", response.StatusCode, string(bodyBytes))
	}
	return nil
}
