package main

import (
	"flag"
	"fmt"
	"os"

	"git.fedesito.me/fedes1to/eserve/cmd/epull/api"
	"git.fedesito.me/fedes1to/eserve/cmd/epull/clientConfig"
	"git.fedesito.me/fedes1to/eserve/internal/cli"
)

func parseRegister() (error, int) {
	// registration
	token := flag.String("token", "", "[REQUIRED FOR REGISTER], Token for registration")
	server := flag.String("server", "", "[REQUIRED FOR REGISTER] Address where eserve is running")
	flavor := flag.String("flavor", "", "Flavor name used on provisioning, will default to hostname")
	insecure := flag.Bool("insecure", false, "Allow insecure requests, DO NOT USE OUTSIDE OF TESTING")

	flag.Parse()

	if *token == "" || *server == "" {
		return fmt.Errorf("Register requires a token and server"), 2
	}

	if *flavor == "" {
		*flavor, _ = os.Hostname()
	}
	return api.HandleRegistration(*token, *server, *flavor, *insecure)
}

func parseProvision() (error, int) {
	flavor := flag.String("flavor", "", "Flavor name used on provisioning, will default to hostname")
	insecure := flag.Bool("insecure", false, "Allow insecure requests, DO NOT USE OUTSIDE OF TESTING")
	flag.Parse()
	if err := api.InitializeMtlsClient(*insecure); err != nil {
		return err, 1
	}

	return api.HandleProvision(*flavor)
}

func main() {
	if len(os.Args) < 2 {
		cli.PrintUsage("epull", []cli.Command{
			{Name: "register", Description: "Set up a flavor (chroot) on the target server"},
		})
		os.Exit(2)
	}

	if err := clientConfig.LoadClientSettings(); err != nil {
		fmt.Fprintln(os.Stderr, "Fail to load settings,", err)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "register":
		err, exitCode := parseRegister()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Registration went wrong,", err)
		}
		os.Exit(exitCode)
	case "provision":
		err, exitCode := parseProvision()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Provision went wrong,", err)
		}
		os.Exit(exitCode)
	}
}
