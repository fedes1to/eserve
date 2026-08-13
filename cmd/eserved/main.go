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
	address := flag.String("ip", "127.0.0.1:8080", "Listen address")
	flag.Parse()

	serveHTTP(*address)
}

func main() {
	log.Println("Starting eserved...")

	// populate necessary global vars
	if certError := serverConfig.InitializeCaCertificate(); certError != nil {
		fmt.Fprintln(os.Stderr, "Failed to init ca certificate,", certError)
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
