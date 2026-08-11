package main

import (
	"time"
)

// these are pretty much only for logging the tokens and registering
type TokenEntry struct {
	Token     string    `json:"token"`
	CN        string    `json:"cn"`
	CreatedAt time.Time `json:"created"`
	UsedAt    time.Time `json:"used"`
}

type TokensFile struct {
	Tokens map[string]TokenEntry `json:"tokens"`
}
