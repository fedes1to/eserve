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
	Subarch    string `json:"subarch"`
	Profile    string `json:"profile"`
	Stagefile  string `json:"stagefile"`
	Flavor     string `json:"flavor"`
}

type ProvisionResponse struct {
	JobID      string `json:"job_id"`
	BinhostURL string `json:"binhost_url"`
}
