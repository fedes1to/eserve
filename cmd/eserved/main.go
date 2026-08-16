package main

import (
	"flag"
	"log"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/chroot"
	serverConfig "git.fedesito.me/fedes1to/eserve/cmd/eserved/config"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
	"git.fedesito.me/fedes1to/eserve/internal/cli"
)

func parseArgs() {
	admin := flag.Bool("admin", true, "allow eservectl endpoint, defaults to true")
	flag.Parse()

	serveHTTP(*admin)
}

func main() {
	log.Println("Starting eserved...")

	steps := []cli.InitStep{
		{Name: "settings", Function: serverConfig.InitializeServerSettings},
		{Name: "stage", Function: storage.InitializeStageFolder},
		{Name: "gcc info", Function: chroot.InitializeGccInfo},
		{Name: "ca certificate", Function: storage.LoadCaCertificate},
		{Name: "tls certificate", Function: storage.LoadTlsCertificates},
		{Name: "tokens", Function: storage.LoadTokens},
		{Name: "machines", Function: storage.LoadMachines},
	}

	cli.MustInit(steps)

	parseArgs()
}
