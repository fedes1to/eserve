package protocols

type IdentificationRequest struct {
	Csr string `json:"csr"`
}

type IdentificationResponse struct {
	Certificate string `json:"cert"`
	CA          string `json:"ca"`
	CN          string `json:"cn"`
	ValidUntil  string `json:"valid_until"`
}

type ProvisionRequest struct {
	SubArch    string `json:"sub_arch"`
	Flavor     string `json:"flavor"`
	UploadPkgs bool   `json:"upload_pkgs"`
}

type ProvisionResponse struct {
}
