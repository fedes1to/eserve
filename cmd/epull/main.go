package main

import (
	"flag"
	"fmt"
	"os"

	clientConfig "git.fedesito.me/fedes1to/eserve/cmd/epull/config"
)

func parseArgs() (error, int) {
	// registration
	register := flag.Bool("register", false, "Sets up a flavor (chroot) on target server")
	token := flag.String("token", "", "[REQUIRED FOR REGISTER], Token for registration")
	server := flag.String("server", "", "[REQUIRED FOR REGISTER] Address where eserve is running")
	flavor := flag.String("flavor", "", "Flavor name used on registration, will default to hostname")
	insecure := flag.Bool("insecure", false, "Allow insecure requests, DO NOT USE OUTSIDE OF TESTING, only applies on register")

	flag.Parse()

	if *register && (*token == "" || *server == "") {
		return fmt.Errorf("Register requires a token and server"), 2
	} else if *register {
		if *flavor == "" {
			*flavor, _ = os.Hostname()
		}
		return handleRegistration(*token, *server, *flavor, *insecure)
	} else {
		if configError := clientConfig.LoadClientSettings(); configError != nil {
			return configError, 1
		}
		if mtlsError := initializeMtlsClient(*insecure); mtlsError != nil {
			return mtlsError, 1
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
