package protocol

type JobInfo struct {
	ID         string `json:"id"`
	CN         string `json:"cn"`
	Flavor     string `json:"flavor,omitempty"`
	Kind       string `json:"kind"`
	State      string `json:"state"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	Terminal   string `json:"terminal,omitempty"`
}

type JobListResponse struct {
	Jobs []JobInfo `json:"jobs"`
}
