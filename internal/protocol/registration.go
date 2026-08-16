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
	GccMachine string `json:"gcc_machine"`
	Subarch    string `json:"sub_arch"`
	Profile    string `json:"profile"`
	Libc       string `json:"libc"`
	Flavor     string `json:"flavor"`
}

type ProvisionResponse struct {
	JobID      string `json:"job_id,omitempty"`
	BinhostURL string `json:"binhost_url"`
	NewChroot  bool   `json:"new_chroot"`
}
