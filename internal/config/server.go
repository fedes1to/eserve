package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type ServerSettings struct {
	ListenAddr     string `json:"listen_addr"`
	BuildThreads   string `json:"build_threads"`
	PerUserThreads string `json:"per_user_threads"`
	ChrootBase     string `json:"chroot_base"`
}
