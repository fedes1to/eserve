package main

import (
	"flag"
	"log"

	serverConfig "git.fedesito.me/fedes1to/eserve/cmd/eserved/config"
	"git.fedesito.me/fedes1to/eserve/internal/macros"
)

func parseArgs() {
	admin := flag.Bool("admin", true, "allow eservectl endpoint, defaults to true")
	flag.Parse()

	serveHTTP(*admin)
}

func main() {
	log.Println("Starting eserved...")

	steps := []macros.InitStep{
		{Name: "settings", Function: serverConfig.InitializeServerSettings},
		{Name: "sysinfo", Function: serverConfig.LoadServerSysinfo},
		{Name: "ca certificate", Function: serverConfig.LoadCaCertificate},
		{Name: "tls certificate", Function: serverConfig.LoadTlsCertificates},
		{Name: "tokens", Function: serverConfig.LoadTokens},
		{Name: "machines", Function: serverConfig.LoadMachines},
	}

	macros.MustInit(steps)

	parseArgs()
}
