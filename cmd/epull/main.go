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
	stage := fs.String("stage", "", "Stage3 file used on provisioning")
	insecure := fs.Bool("insecure", false, "Allow insecure requests, DO NOT USE OUTSIDE OF TESTING")

	fs.Parse(os.Args[2:])

	if *token == "" || *server == "" {
		return fmt.Errorf("Register requires a token and server"), 2
	}

	if *flavor == "" {
		*flavor, _ = os.Hostname()
	}
	return api.HandleRegistration(*token, *server, *flavor, *stage, *insecure)
}

func parseProvision() (error, int) {
	fs := flag.NewFlagSet("provision", flag.ExitOnError)
	flavor := fs.String("flavor", "", "Flavor name used on provisioning, empty uses previous flavor")
	token := fs.String("token", "", "Token required when switching flavor")
	insecure := fs.Bool("insecure", false, "Allow insecure requests, DO NOT USE OUTSIDE OF TESTING")
	stage := fs.String("stage", "", "Stage3 file used on provisioning")
	fs.Parse(os.Args[2:])

	if err := clientConfig.LoadClientSettings(); err != nil {
		return err, 1
	}
	if err := api.InitializeMtlsClient(*insecure); err != nil {
		return err, 1
	}

	if *stage == "" {
		var err error
		*stage, err = api.AskStagefile()
		if err != nil {
			return err, 1
		}
	}

	return api.HandleProvision(*flavor, *stage, *token)
}

func parseSync() (error, int) {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	insecure := fs.Bool("insecure", false, "Allow insecure requests, DO NOT USE OUTSIDE OF TESTING")
	assumeYes := fs.Bool("y", false, "Sync without prompting when out of date")
	fs.Parse(os.Args[2:])

	if err := clientConfig.LoadClientSettings(); err != nil {
		return err, 1
	}
	if err := api.InitializeMtlsClient(*insecure); err != nil {
		return err, 1
	}

	return api.HandleSync(*assumeYes, *insecure)
}

func parseSelfUpdate() (error, int) {
	fs := flag.NewFlagSet("selfupdate", flag.ExitOnError)
	insecure := fs.Bool("insecure", false, "Allow insecure requests, DO NOT USE OUTSIDE OF TESTING")
	fs.Parse(os.Args[2:])

	if err := clientConfig.LoadClientSettings(); err != nil {
		return err, 1
	}
	if err := api.InitializeMtlsClient(*insecure); err != nil {
		return err, 1
	}

	return api.HandleSelfUpdate()
}

func main() {
	if len(os.Args) < 2 {
		cli.PrintUsage("epull", []cli.Command{
			{Name: "register", Description: "Set up a flavor (chroot) on the target server"},
			{Name: "provision", Description: "Provisions your machine on the target server"},
			{Name: "sync", Description: "Sync your portage config with the server flavor"},
			{Name: "selfupdate", Description: "Replace this epull binary with the server-hosted build"},
		})
		os.Exit(2)
	}

	switch os.Args[1] {
	case "register":
		err, exitCode := parseRegister()
		if err != nil {
			fmt.Fprintln(os.Stderr, "\x1b[31mRegistration went wrong,\x1b[0m", err)
		}
		os.Exit(exitCode)
	case "provision":
		err, exitCode := parseProvision()
		if err != nil {
			fmt.Fprintln(os.Stderr, "\x1b[31mProvision went wrong,\x1b[0m", err)
		}
		os.Exit(exitCode)
	case "sync":
		err, exitCode := parseSync()
		if err != nil {
			fmt.Fprintln(os.Stderr, "\x1b[31mSync went wrong,\x1b[0m", err)
		}
		os.Exit(exitCode)
	case "selfupdate":
		err, exitCode := parseSelfUpdate()
		if err != nil {
			fmt.Fprintln(os.Stderr, "\x1b[31mSelfupdate went wrong,\x1b[0m", err)
		}
		os.Exit(exitCode)
	}
}
