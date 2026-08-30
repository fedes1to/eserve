package storage

import (
	"os"
	"path/filepath"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/config"
)

type FlavorFingerprint struct {
	Fingerprint string    `json:"fingerprint"`
	SyncedBy    string    `json:"synced_by"`
	SyncedAt    time.Time `json:"synced_at"`
}

func flavorFingerprintPath(flavor string) string {
	return filepath.Join(serverConfig.ServerConfigPath, "sync", flavor, "fingerprint.json")
}

// the whole fingerprint record (who synced the flavor and when, not
// just the fingerprint itself)
func FlavorFingerprintInfo(flavor string) (fp FlavorFingerprint, ok bool) {
	if err := config.LoadJsonFile(flavorFingerprintPath(flavor), &fp); err != nil {
		return fp, false
	}
	return fp, fp.Fingerprint != ""
}

func GetFlavorFingerprint(flavor string) (fingerprint string, ok bool) {
	fp, ok := FlavorFingerprintInfo(flavor)
	return fp.Fingerprint, ok
}

func SetFlavorFingerprint(flavor, fingerprint, syncedBy string) error {
	syncDir := filepath.Join(serverConfig.ServerConfigPath, "sync", flavor)
	if err := os.MkdirAll(syncDir, 0700); err != nil {
		return err
	}
	return config.SafeSaveJsonFile(flavorFingerprintPath(flavor), FlavorFingerprint{
		Fingerprint: fingerprint,
		SyncedBy:    syncedBy,
		SyncedAt:    time.Now(),
	})
}
