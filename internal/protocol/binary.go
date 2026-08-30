package protocol

type BinaryManifest struct {
	Name       string `json:"name"`
	Arch       string `json:"arch"`
	SHA256     string `json:"sha256"`
	Size       int64  `json:"size"`
	UploadedAt string `json:"uploaded_at"`
}

const MaxBinarySize = 512 << 20

type BinaryListResponse struct {
	Binaries []BinaryManifest `json:"binaries"`
}
