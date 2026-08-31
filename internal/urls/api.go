package urls

const (
	IdentitySuburl       = "/api/v1/identity"
	CaSuburl             = "/api/v1/ca"
	ProvisionSuburl      = "/api/v1/provision"
	SyncSuburl           = "/api/v1/sync"
	CheckSyncSuburl      = "/api/v1/sync/check"
	StagesSuburl         = "/api/v1/stages"
	JobsCancelSuburl     = "/api/v1/jobs/cancel"
	JobsStreamSuburl     = "/api/v1/jobs/stream"
	BinarySuburl         = "/api/v1/binary"
	BinaryManifestSuburl = "/api/v1/binary/manifest"
	SigningKeySuburl     = "/api/v1/signing-key"
	// portage binhost, served to plain https clients (no mTLS, portage can't do it)
	PkgsSuburl = "/pkgs"
)
