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
	SubArch string `json:"sub_arch"`
	Flavor  string `json:"flavor"`
}

type ProvisionResponse struct {
}
