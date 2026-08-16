package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func parseArgs() error {
	fs := flag.NewFlagSet("eserved", flag.ExitOnError)
	admin := fs.Bool("admin", true, "allow eservectl endpoint, defaults to true")
	fs.Parse(os.Args[1:])

	return serveHTTP(*admin)
}

func main() {
	log.Println("Starting eserved...")

	if err := parseArgs(); err != nil {
		fmt.Fprintln(os.Stderr, "Something went wrong,", err)
		os.Exit(1)
	}
}
