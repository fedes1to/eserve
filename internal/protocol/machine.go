package protocol

type RevokeRequest struct {
	CN string `json:"cn"`
}

type DeleteMachineRequest struct {
	CN string `json:"cn"`
}

type DeleteTokenRequest struct {
	Token string `json:"token"`
}
