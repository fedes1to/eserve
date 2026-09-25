package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

type ClientIdentity struct {
	CN          string
	Fingerprint string
}

type ctxKey string

const CtxKeyIdentity ctxKey = "client_identity"

// caps the body and decodes it, answering the request itself when it can't
func decodeJSONBody(w http.ResponseWriter, r *http.Request, target any, what string) bool {
	r.Body = http.MaxBytesReader(w, r.Body, protocol.MaxJSONBodySize)
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return false
		}
		http.Error(w, "couldn't decode "+what, http.StatusBadRequest)
		return false
	}
	return true
}

func ClientIP(r *http.Request) string {
	proxyIpHeader := r.Header.Get("X-Forwarded-For")

	if proxyIpHeader == "" {
		return r.RemoteAddr
	}

	return strings.Split(proxyIpHeader, ",")[0] + " (via " + r.RemoteAddr + ")"
}
