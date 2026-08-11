package main

import (
	"flag"
	"log"
)

func parseArgs() {
	address := flag.String("ip", "127.0.0.1:8080", "Listen address")
	flag.Parse()

	serveHTTP(*address)
}

func main() {
	log.Println("Starting eserved...")
	parseArgs()
}
