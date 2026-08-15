package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"git.fedesito.me/fedes1to/eserve/internal/macros"
)

func parseTokenFlags() (error, int) {
	create := flag.Bool("create", false, "requests a token to be created on eserved")
	flag.Parse()

	connectError := tryConnect()
	if connectError != nil {
		return fmt.Errorf("Can't connect, %w", connectError), 0
	}

	if *create {
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
	if len(os.Args) < 2 {
		macros.PrintUsage("epull", []macros.Command{
			{Name: "token", Description: "Generate a token used for registration"},
		})
		os.Exit(2)
	}

	switch os.Args[1] {
	case "token":
		if parseError, exitCode := parseTokenFlags(); parseError != nil {
			fmt.Fprintln(os.Stderr, "Something went wrong,", parseError)
			os.Exit(exitCode)
		}
	}

}
