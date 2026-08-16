package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"git.fedesito.me/fedes1to/eserve/cmd/eservectl/admin"
	"git.fedesito.me/fedes1to/eserve/cmd/eservectl/storage"
	"git.fedesito.me/fedes1to/eserve/internal/cli"
)

func parseTokenFlags() (error, int) {
	create := flag.Bool("create", false, "requests a token to be created on eserved")
	flag.Parse()

	connectError := admin.TryConnect()
	if connectError != nil {
		return fmt.Errorf("Can't connect, %w", connectError), 0
	}

	if *create {
		token, createError := admin.CreateToken()
		if createError != nil {
			return fmt.Errorf("Couldn't create token, %w", createError), 1
		}
		log.Println("New token:", token)
		return nil, 0
	}

	return nil, 0
}

func parseStage() (error, int) {
	if len(os.Args) < 3 {
		cli.PrintUsage("epull stage", []cli.Command{
			{Name: "download", Description: "Downloads a stage from the internet for eserved to use"},
			{Name: "install", Description: "Installs a local stage for eserved to use"},
		})
		os.Exit(2)
	}

	switch os.Args[2] {
	case "download":
		url := flag.String("url", "", "url for the stagefile to download")
		flag.Parse()
		if *url == "" {
			return fmt.Errorf("URL must be provided for download"), 2
		}
		fileName, downloadError := storage.DownloadStage(*url)
		if downloadError != nil {
			return downloadError, 1
		}
		sha256sum, shaError := storage.GetStageHash(fileName)
		if shaError != nil {
			return fmt.Errorf("Download succeeded but couldn't get SHA256 hash, %w", shaError), 1
		}
		log.Printf("sha256sum of %v: %v", fileName, sha256sum)

	case "install":
		path := flag.String("path", "", "local path for the stagefile to install")
		flag.Parse()
		if installError := storage.InstallStage(*path); installError != nil {
			return installError, 1
		}
	}

	return nil, 0
}

func main() {
	if len(os.Args) < 2 {
		cli.PrintUsage("epull", []cli.Command{
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
	case "stage":
		if parseError, exitCode := parseTokenFlags(); parseError != nil {
			fmt.Fprintln(os.Stderr, "Something went wrong,", parseError)
			os.Exit(exitCode)
		}
	}

}
