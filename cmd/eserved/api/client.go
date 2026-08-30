package api

import (
	"net/http"
	"strings"
)

type ClientIdentity struct {
	CN          string
	Fingerprint string
}

type ctxKey string

const CtxKeyIdentity ctxKey = "client_identity"

func ClientIP(r *http.Request) string {
	proxyIpHeader := r.Header.Get("X-Forwarded-For")

	if proxyIpHeader == "" {
		return r.RemoteAddr
	}

	return strings.Split(proxyIpHeader, ",")[0] + " (via " + r.RemoteAddr + ")"
}
