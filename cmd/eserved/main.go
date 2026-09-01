package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
)

func parseArgs() error {
	fs := flag.NewFlagSet("eserved", flag.ExitOnError)
	admin := fs.Bool("admin", true, "allow eservectl endpoint, defaults to true")
	pidfile := fs.String("pidfile", "", "path to write the pid file to")
	fs.Parse(os.Args[1:])

	if *pidfile != "" {
		if err := os.WriteFile(*pidfile, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
			return err
		}
	}

	return serveHTTP(*admin)
}

func main() {
	log.Println("Starting eserved...")

	if err := parseArgs(); err != nil {
		fmt.Fprintln(os.Stderr, "Something went wrong,", err)
		os.Exit(1)
	}
}
