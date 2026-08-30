package admin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"git.fedesito.me/fedes1to/eserve/internal/protocol"
	"git.fedesito.me/fedes1to/eserve/internal/urls"
)

// hash first (the manifest carries the sha), then stream it multipart without buffering
func PostUploadBinary(name, arch, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	file.Close()
	file, err = os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	manifest := protocol.BinaryManifest{
		Name:   name,
		Arch:   arch,
		SHA256: hex.EncodeToString(hasher.Sum(nil)),
	}
	manifestJSON, _ := json.Marshal(manifest)

	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	go func() {
		defer pipeWriter.Close()
		if err := multipartWriter.WriteField("manifest", string(manifestJSON)); err != nil {
			return
		}
		part, err := multipartWriter.CreateFormFile("binary", filepath.Base(path))
		if err != nil {
			return
		}
		io.Copy(part, file)
		multipartWriter.Close()
	}()

	request, err := http.NewRequest("POST", urls.SocketURL+urls.BinaryUploadSuburl, pipeReader)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	response, err := adminClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	bodyBytes, _ := io.ReadAll(response.Body)
	if response.StatusCode != 200 {
		return fmt.Errorf("couldn't upload binary, code %v, body:\n%v", response.StatusCode, string(bodyBytes))
	}
	return nil
}

func PostListBinaries() (protocol.BinaryListResponse, error) {
	response, err := adminClient.Post(urls.SocketURL+urls.BinaryListSuburl, "", nil)
	if err != nil {
		return protocol.BinaryListResponse{}, err
	}
	defer response.Body.Close()

	var list protocol.BinaryListResponse
	if err := json.NewDecoder(response.Body).Decode(&list); err != nil {
		return protocol.BinaryListResponse{}, err
	}
	if response.StatusCode != 200 {
		return protocol.BinaryListResponse{}, fmt.Errorf("couldn't list binaries, code %v", response.StatusCode)
	}
	return list, nil
}
