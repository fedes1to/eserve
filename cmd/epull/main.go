package main

import (
	"flag"
	"fmt"
	"os"

	"git.fedesito.me/fedesito/eserve/internal/config"
)

func parseArgs() (error, int) {
	// registration
	register := flag.Bool("register", false, "Sets up a flavor (chroot) on target server")
	token := flag.String("token", "", "[REQUIRED FOR REGISTER], Token for registration")
	server := flag.String("server", "", "[REQUIRED FOR REGISTER] Address where eserve is running")
	flavor := flag.String("flavor", "", "Flavor name used on registration, will default to hostname")

	flag.Parse()

	if *register && (*token == "" || *server == "") {
		return fmt.Errorf("Register requires a token and server"), 2
	} else if *register {
		if *flavor == "" {
			*flavor, _ = os.Hostname()
		}
		return handleRegistration(*token, *server, *flavor)
	} else {
		if configError := config.LoadClientSettings(); configError != nil {
			return configError, 1
		}
	}

	return nil, 0

}

func main() {
	err, exitCode := parseArgs()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Something went wrong,", err)
	}
	os.Exit(exitCode)
}
