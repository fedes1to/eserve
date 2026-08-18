package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"git.fedesito.me/fedes1to/eserve/cmd/eservectl/admin"
	"git.fedesito.me/fedes1to/eserve/cmd/eservectl/storage"
	"git.fedesito.me/fedes1to/eserve/internal/cli"
	"git.fedesito.me/fedes1to/eserve/internal/sharedStorage"
)

func parseTokenFlags() (error, int) {
	if len(os.Args) < 3 || strings.Contains(os.Args[2], "help") {
		cli.PrintUsage("epull token", []cli.Command{
			{Name: "create", Description: "Creates a token on eserved"},
		})
		os.Exit(2)
	}

	err := admin.TryConnect()
	if err != nil {
		return fmt.Errorf("Can't connect, %w", err), 0
	}

	switch os.Args[2] {
	case "create":
		token, err := admin.PostCreateToken()
		if err != nil {
			return fmt.Errorf("Couldn't create token, %w", err), 1
		}
		log.Println("New token:", token)
	}

	return nil, 0
}

func parseStage() (error, int) {
	if len(os.Args) < 3 || strings.Contains(os.Args[2], "help") {
		cli.PrintUsage("epull stage", []cli.Command{
			{Name: "download", Description: "Downloads a stage from the internet for eserved to use"},
			{Name: "install", Description: "Installs a local stage for eserved to use"},
			{Name: "list", Description: "Lists downloaded stages"},
		})
		os.Exit(2)
	}

	switch os.Args[2] {
	case "download":
		fs := flag.NewFlagSet("stage download", flag.ExitOnError)
		url := fs.String("url", "", "url for the stagefile to download")
		fs.Parse(os.Args[3:])
		if *url == "" {
			return fmt.Errorf("URL must be provided for download"), 2
		}
		fileName, err := storage.DownloadStage(*url)
		if err != nil {
			return err, 1
		}
		sha256sum, err := storage.GetStageHash(fileName)
		if err != nil {
			return fmt.Errorf("Download succeeded but couldn't get SHA256 hash, %w", err), 1
		}
		log.Printf("sha256sum of %v: %v", fileName, sha256sum)

	case "install":
		fs := flag.NewFlagSet("stage install", flag.ExitOnError)
		path := fs.String("path", "", "local path for the stagefile to install")
		fs.Parse(os.Args[3:])
		if err := storage.InstallStage(*path); err != nil {
			return err, 1
		}
	case "list":
		stageList, err := sharedStorage.GetStageList()
		if err != nil {
			return fmt.Errorf("Couldn't list stages, %w", err), 1
		}
		for _, stage := range stageList {
			fmt.Println(stage)
		}
	}

	return nil, 0
}

func main() {
	if len(os.Args) < 2 || strings.Contains(os.Args[1], "help") {
		cli.PrintUsage("epull", []cli.Command{
			{Name: "token", Description: "Manage tokens used for registration"},
			{Name: "stage", Description: "Manage stage3 files"},
		})
		os.Exit(2)
	}

	switch os.Args[1] {
	case "token":
		if err, exitCode := parseTokenFlags(); err != nil {
			fmt.Fprintln(os.Stderr, "Something went wrong,", err)
			os.Exit(exitCode)
		}
	case "stage":
		if err, exitCode := parseStage(); err != nil {
			fmt.Fprintln(os.Stderr, "Something went wrong,", err)
			os.Exit(exitCode)
		}
	}

}
