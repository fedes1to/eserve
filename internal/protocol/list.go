package protocol

import "time"

type MachineInfo struct {
	CN          string    `json:"cn"`
	Subarch     string    `json:"march"`
	Profile     string    `json:"profile"`
	Flavor      string    `json:"flavor"`
	Fingerprint string    `json:"fingerprint"`
	RevokedAt   time.Time `json:"revoked_at"`
}

type MachineListResponse struct {
	Machines []MachineInfo `json:"machines"`
}

type TokenInfo struct {
	Token     string    `json:"token"`
	CN        string    `json:"cn"`
	CreatedAt time.Time `json:"created"`
	UsedAt    time.Time `json:"used"`
}

type TokenListResponse struct {
	Tokens []TokenInfo `json:"tokens"`
}
