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
	fs := flag.NewFlagSet("register", flag.ExitOnError)
	// registration
	token := fs.String("token", "", "[REQUIRED FOR REGISTER], Token for registration")
	server := fs.String("server", "", "[REQUIRED FOR REGISTER] Address where eserve is running")
	flavor := fs.String("flavor", "", "Flavor name used on provisioning, will default to hostname")
	insecure := fs.Bool("insecure", false, "Allow insecure requests, DO NOT USE OUTSIDE OF TESTING")

	fs.Parse(os.Args[2:])

	if *token == "" || *server == "" {
		return fmt.Errorf("Register requires a token and server"), 2
	}

	if *flavor == "" {
		*flavor, _ = os.Hostname()
	}
	return api.HandleRegistration(*token, *server, *flavor, *insecure)
}

func parseProvision() (error, int) {
	fs := flag.NewFlagSet("provision", flag.ExitOnError)
	flavor := fs.String("flavor", "", "Flavor name used on provisioning, will default to hostname")
	insecure := fs.Bool("insecure", false, "Allow insecure requests, DO NOT USE OUTSIDE OF TESTING")
	fs.Parse(os.Args[2:])

	if err := clientConfig.LoadClientSettings(); err != nil {
		return err, 1
	}
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
