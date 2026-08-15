package protocol

type IdentificationRequest struct {
	Csr string `json:"csr"`
}

type IdentificationResponse struct {
	Certificate string `json:"cert"`
	CN          string `json:"cn"`
	ValidUntil  string `json:"valid_until"`
}

type ProvisionRequest struct {
	Arch    string `json:"arch"`
	Subarch string `json:"sub_arch"`
	Libc    string `json:"libc"`
	Flavor  string `json:"flavor"`
}

type ProvisionResponse struct {
	JobID      string `json:"job_id,omitempty"`
	BinhostURL string `json:"binhost_url"`
	NewChroot  bool   `json:"new_chroot"`
}
