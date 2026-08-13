package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	serverConfig "git.fedesito.me/fedes1to/eserve/cmd/eserved/config"
	"git.fedesito.me/fedes1to/eserve/internal/config"
)

func parseArgs() {
	admin := flag.Bool("admin", true, "allow eservectl endpoint, defaults to true")
	flag.Parse()

	serveHTTP(*admin)
}

func main() {
	log.Println("Starting eserved...")

	if initError := serverConfig.InitializeServerSettings(); initError != nil {
		fmt.Fprintln(os.Stderr, "Failed to generate settings.json,", initError)
		os.Exit(1)
	}

	// populate necessary global vars
	if certError := serverConfig.LoadCaCertificate(); certError != nil {
		fmt.Fprintln(os.Stderr, "Failed to init ca certificate,", certError)
		os.Exit(1)
	}
	if tlsError := serverConfig.LoadTlsCertificates(); tlsError != nil {
		fmt.Fprintln(os.Stderr, "Failed to init tls certificate,", tlsError)
		os.Exit(1)
	}
	if tokenError := config.LoadTokens(); tokenError != nil {
		fmt.Fprintln(os.Stderr, "Failed to populate tokens,", tokenError)
		os.Exit(1)
	}
	if machineError := config.LoadMachines(); machineError != nil {
		fmt.Fprintln(os.Stderr, "Failed to populate machines,", machineError)
		os.Exit(1)
	}
	parseArgs()
}
