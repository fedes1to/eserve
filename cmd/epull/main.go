package main

import (
	"flag"
	"fmt"
	"os"

	"git.fedesito.me/fedes1to/eserve/internal/macros"
)

func parseRegister() (error, int) {
	// registration
	token := flag.String("token", "", "[REQUIRED FOR REGISTER], Token for registration")
	server := flag.String("server", "", "[REQUIRED FOR REGISTER] Address where eserve is running")
	flavor := flag.String("flavor", "", "Flavor name used on registration, will default to hostname")
	insecure := flag.Bool("insecure", false, "Allow insecure requests, DO NOT USE OUTSIDE OF TESTING")

	flag.Parse()

	if *token == "" || *server == "" {
		return fmt.Errorf("Register requires a token and server"), 2
	}

	if *flavor == "" {
		*flavor, _ = os.Hostname()
	}
	return handleRegistration(*token, *server, *flavor, *insecure)
}

func main() {
	if len(os.Args) < 2 {
		macros.PrintUsage("epull", []macros.Command{
			{Name: "register", Description: "Set up a flavor (chroot) on the target server"},
		})
		os.Exit(2)
	}

	switch os.Args[1] {
	case "register":
		registerError, exitCode := parseRegister()
		if registerError != nil {
			fmt.Fprintln(os.Stderr, "Registration went wrong,", registerError)
		}
		os.Exit(exitCode)
	}

}
