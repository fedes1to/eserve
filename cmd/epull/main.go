package main

import (
	"flag"
	"fmt"
	"git.fedesito.me/fedesito/eserve/internal/config"
	"log"
	"os"
)

func parseArgs() {
	// registration
	register := flag.Bool("register", false, "Sets up a profile (chroot) on target server")
	token := flag.String("token", "", "[REQUIRED FOR REGISTER], Token for registration")
	server := flag.String("server", "", "[REQUIRED FOR REGISTER] Address where eserve is running")
	flavor := flag.String("flavor", "", "Flavor name used on registration, will default to hostname")

	flag.Parse()

	if *register && (*token == "" || *server == "") {
		log.Fatalln("Register requires a token and server")
	} else if *register {
		if *flavor == "" {
			*flavor, _ = os.Hostname()
		}
		handleRegistration(*token, *server, *flavor)
	} else {
		if configError := config.LoadClient(); configError != nil {
			fmt.Errorf("Couldn't load config, %w", configError)
		}
	}

}

func main() {
	parseArgs()
}
