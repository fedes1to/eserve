package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func parseFlags() (error, int) {
	createtoken := flag.Bool("createtoken", false, "requests a token to be created on eserved")
	flag.Parse()

	connectError := tryConnect()
	if connectError != nil {
		return fmt.Errorf("Can't connect, %w", connectError), 0
	}

	if *createtoken {
		token, createError := createToken()
		if createError != nil {
			return fmt.Errorf("Couldn't create token, %w", createError), 1
		}
		log.Println("New token:", token)
		return nil, 0
	}

	return nil, 0
}

func main() {

	if parseError, exitCode := parseFlags(); parseError != nil {
		fmt.Fprintln(os.Stderr, "Something went wrong,", parseError)
		os.Exit(exitCode)
	}
}
