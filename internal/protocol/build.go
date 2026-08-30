package protocol

type BuildRequest struct {
	Flavor   string   `json:"flavor"`
	Packages []string `json:"packages"`
}

type BuildResponse struct {
	JobID string `json:"job_id"`
}
