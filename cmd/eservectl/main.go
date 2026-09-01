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

func printMainUsage() {
	cli.PrintUsage("eservectl", []cli.Command{
		{Name: "token", Description: "Manage tokens used for registration"},
		{Name: "stage", Description: "Manage stage3 files"},
		{Name: "machine", Description: "Manage machines registered on eserved"},
		{Name: "job", Description: "List, cancel and stream eserved's jobs"},
		{Name: "build", Description: "Build packages in a flavor's chroot"},
		{Name: "flavor", Description: "Manage flavor portage configs"},
		{Name: "binary", Description: "Manage the binaries the server hands out"},
	})
	os.Exit(2)
}

func printTokenUsage() {
	cli.PrintUsage("eservectl token", []cli.Command{
		{Name: "create", Description: "Creates a token on eserved"},
		{Name: "list", Description: "Lists the tokens on eserved"},
		{Name: "delete", Description: "Deletes a token on eserved"},
	})
	os.Exit(2)
}

func printStageUsage() {
	cli.PrintUsage("eservectl stage", []cli.Command{
		{Name: "download", Description: "Downloads a stage from the internet for eserved to use"},
		{Name: "install", Description: "Installs a local stage for eserved to use"},
		{Name: "list", Description: "Lists downloaded stages"},
	})
	os.Exit(2)
}

func printMachineUsage() {
	cli.PrintUsage("eservectl machine", []cli.Command{
		{Name: "revoke", Description: "Revokes a machine on eserved"},
		{Name: "list", Description: "Lists the machines on eserved"},
		{Name: "delete", Description: "Deletes a machine on eserved (and its stored sync)"},
	})
	os.Exit(2)
}

func printJobUsage() {
	cli.PrintUsage("eservectl job", []cli.Command{
		{Name: "list", Description: "Lists every job eserved knows about"},
		{Name: "cancel", Description: "Cancels a running job (-id)"},
		{Name: "stream", Description: "Streams a job's events, replaying the past ones (-id)"},
	})
	os.Exit(2)
}

func printBuildUsage() {
	cli.PrintUsage("eservectl build", []cli.Command{
		{Name: "start", Description: "Starts a build job: -flavor and -package (repeatable)"},
	})
	os.Exit(2)
}

func printFlavorUsage() {
	cli.PrintUsage("eservectl flavor", []cli.Command{
		{Name: "apply", Description: "Applies the flavor's config to its chroots (-flavor)"},
		{Name: "config", Description: "Shows the flavor's config files (-flavor)"},
		{Name: "config create", Description: "Scaffolds a flavor's config files (-flavor)"},
	})
	os.Exit(2)
}

func printBinaryUsage() {
	cli.PrintUsage("eservectl binary", []cli.Command{
		{Name: "upload", Description: "Uploads a binary: -name, -arch, -file"},
		{Name: "list", Description: "Lists the binaries the server has"},
	})
	os.Exit(2)
}

func parseTokenFlags() (error, int) {
	if len(os.Args) < 3 || strings.Contains(os.Args[2], "help") ||
		(os.Args[2] != "create" && os.Args[2] != "list" && os.Args[2] != "delete") {
		printTokenUsage()
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
	case "list":
		list, err := admin.PostListTokens()
		if err != nil {
			return fmt.Errorf("Couldn't list tokens, %w", err), 1
		}
		if len(list) == 0 {
			fmt.Println("no tokens")
			return nil, 0
		}
		fmt.Printf("%-28s %-16s %-19s %s\n", "TOKEN", "CN", "CREATED", "USED")
		for _, token := range list {
			used := "-"
			if !token.UsedAt.IsZero() {
				used = token.UsedAt.Format("2006-01-02 15:04:05")
			}
			fmt.Printf("%-28s %-16s %-19s %s\n", token.Token, token.CN, token.CreatedAt.Format("2006-01-02 15:04:05"), used)
		}
	case "delete":
		fs := flag.NewFlagSet("token delete", flag.ExitOnError)
		token := fs.String("token", "", "the token to delete")
		fs.Parse(os.Args[3:])
		if *token == "" {
			return fmt.Errorf("-token flag is required"), 2
		}
		err = admin.PostDeleteToken(*token)
		if err != nil {
			return fmt.Errorf("Couldn't delete token, %w", err), 1
		}
	}

	return nil, 0
}

func parseStageFlags() (error, int) {
	if len(os.Args) < 3 || strings.Contains(os.Args[2], "help") ||
		(os.Args[2] != "download" && os.Args[2] != "install" && os.Args[2] != "list") {
		printStageUsage()
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

func parseMachineFlags() (error, int) {
	if len(os.Args) < 3 || strings.Contains(os.Args[2], "help") ||
		(os.Args[2] != "revoke" && os.Args[2] != "list" && os.Args[2] != "delete") {
		printMachineUsage()
	}

	err := admin.TryConnect()
	if err != nil {
		return fmt.Errorf("Can't connect, %w", err), 0
	}

	switch os.Args[2] {
	case "revoke":
		fs := flag.NewFlagSet("machine revoke", flag.ExitOnError)
		cn := fs.String("cn", "", "cn (usually hostname) of the machine to revoke")
		fs.Parse(os.Args[3:])
		if *cn == "" {
			return fmt.Errorf("-cn flag is required"), 2
		}
		err = admin.PostRevokeMachine(*cn)
		if err != nil {
			return fmt.Errorf("Couldn't revoke machine, %w", err), 1
		}
	case "list":
		list, err := admin.PostListMachines()
		if err != nil {
			return fmt.Errorf("Couldn't list machines, %w", err), 1
		}
		if len(list) == 0 {
			fmt.Println("no machines")
			return nil, 0
		}
		fmt.Printf("%-16s %-12s %-12s %-40s %-64s %s\n", "CN", "FLAVOR", "MARCH", "PROFILE", "FINGERPRINT", "REVOKED")
		for _, machine := range list {
			revoked := "-"
			if !machine.RevokedAt.IsZero() {
				revoked = machine.RevokedAt.Format("2006-01-02 15:04:05")
			}
			fmt.Printf("%-16s %-12s %-12s %-40s %-64s %s\n", machine.CN, machine.Flavor, machine.Subarch, machine.Profile, machine.Fingerprint, revoked)
		}
	case "delete":
		fs := flag.NewFlagSet("machine delete", flag.ExitOnError)
		cn := fs.String("cn", "", "cn (usually hostname) of the machine to delete")
		fs.Parse(os.Args[3:])
		if *cn == "" {
			return fmt.Errorf("-cn flag is required"), 2
		}
		err = admin.PostDeleteMachine(*cn)
		if err != nil {
			return fmt.Errorf("Couldn't delete machine, %w", err), 1
		}
	}

	return nil, 0
}

func parseJobFlags() (error, int) {
	if len(os.Args) < 3 || strings.Contains(os.Args[2], "help") ||
		(os.Args[2] != "list" && os.Args[2] != "cancel" && os.Args[2] != "stream") {
		printJobUsage()
	}

	err := admin.TryConnect()
	if err != nil {
		return fmt.Errorf("Can't connect, %w", err), 0
	}

	switch os.Args[2] {
	case "list":
		list, err := admin.PostListJobs()
		if err != nil {
			return err, 1
		}
		if len(list.Jobs) == 0 {
			fmt.Println("no jobs")
			return nil, 0
		}
		fmt.Printf("%-34s %-16s %-12s %-10s %-10s %s\n", "ID", "CN", "FLAVOR", "KIND", "STATE", "TERMINAL")
		for _, job := range list.Jobs {
			fmt.Printf("%-34s %-16s %-12s %-10s %-10s %s\n", job.ID, job.CN, job.Flavor, job.Kind, job.State, job.Terminal)
		}
	case "cancel":
		fs := flag.NewFlagSet("job cancel", flag.ExitOnError)
		id := fs.String("id", "", "id of the job to cancel")
		fs.Parse(os.Args[3:])
		if *id == "" {
			return fmt.Errorf("-id flag is required"), 2
		}
		if err := admin.PostCancelJob(*id); err != nil {
			return err, 1
		}
		fmt.Println("job cancelled")
	case "stream":
		fs := flag.NewFlagSet("job stream", flag.ExitOnError)
		id := fs.String("id", "", "id of the job to stream")
		fs.Parse(os.Args[3:])
		if *id == "" {
			return fmt.Errorf("-id flag is required"), 2
		}
		_, success, err := admin.PostJobStream(*id)
		if err != nil {
			return err, 1
		}
		if !success {
			return fmt.Errorf("the job ended with an error"), 1
		}
	}

	return nil, 0
}

// a flag that collects multiple values (-package a -package b)
type stringArray []string

func (s *stringArray) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringArray) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func parseBuildFlags() (error, int) {
	if len(os.Args) < 3 || strings.Contains(os.Args[2], "help") || os.Args[2] != "start" {
		printBuildUsage()
	}

	err := admin.TryConnect()
	if err != nil {
		return fmt.Errorf("Can't connect, %w", err), 0
	}

	fs := flag.NewFlagSet("build start", flag.ExitOnError)
	flavor := fs.String("flavor", "", "flavor to build in")
	var packages stringArray
	fs.Var(&packages, "package", "package to build, repeatable")
	fs.Parse(os.Args[3:])
	if *flavor == "" {
		return fmt.Errorf("-flavor flag is required"), 2
	}
	if len(packages) == 0 {
		return fmt.Errorf("at least one -package flag is required"), 2
	}

	jobID, err := admin.PostStartBuild(*flavor, packages)
	if err != nil {
		return err, 1
	}
	log.Println("started build job", jobID)

	// the build runs in the background, stream it live
	_, success, err := admin.PostJobStream(jobID)
	if err != nil {
		return err, 1
	}
	if !success {
		return fmt.Errorf("the build failed, job %s", jobID), 1
	}
	return nil, 0
}

func parseFlavorFlags() (error, int) {
	if len(os.Args) < 3 || strings.Contains(os.Args[2], "help") ||
		(os.Args[2] != "apply" && os.Args[2] != "config") {
		printFlavorUsage()
	}

	args := os.Args[3:]
	mode := os.Args[2]
	if mode == "config" && len(args) > 0 {
		if strings.Contains(args[0], "help") {
			printFlavorUsage()
		}
		// "config create" is the only two-word subcommand, normalize it
		if args[0] == "create" {
			mode, args = "create", args[1:]
		}
	}

	switch mode {
	case "apply":
		err := admin.TryConnect()
		if err != nil {
			return fmt.Errorf("Can't connect, %w", err), 0
		}
		fs := flag.NewFlagSet("flavor apply", flag.ExitOnError)
		flavor := fs.String("flavor", "", "flavor to apply the config to")
		fs.Parse(args)
		if *flavor == "" {
			return fmt.Errorf("-flavor flag is required"), 2
		}
		if err := admin.PostApplyFlavor(*flavor); err != nil {
			return err, 1
		}
		fmt.Println("flavor config applied")
	case "config":
		fs := flag.NewFlagSet("flavor config", flag.ExitOnError)
		flavor := fs.String("flavor", "", "flavor to show the config of")
		fs.Parse(args)
		if *flavor == "" {
			return fmt.Errorf("-flavor flag is required"), 2
		}
		files, err := storage.ListFlavorConfig(*flavor)
		if err != nil {
			return err, 1
		}
		if len(files) == 0 {
			fmt.Println("flavor has no config yet, create one with 'eservectl flavor config create'")
			return nil, 0
		}
		for _, file := range files {
			fmt.Println(file)
		}
	case "create":
		fs := flag.NewFlagSet("flavor config create", flag.ExitOnError)
		flavor := fs.String("flavor", "", "flavor to create the config for")
		fs.Parse(args)
		if *flavor == "" {
			return fmt.Errorf("-flavor flag is required"), 2
		}
		created, err := storage.CreateFlavorConfig(*flavor)
		if err != nil {
			return err, 1
		}
		if len(created) == 0 {
			fmt.Println("nothing to create, the flavor already has a config")
			return nil, 0
		}
		for _, file := range created {
			fmt.Println("created", file)
		}
	}

	return nil, 0
}

func parseBinaryFlags() (error, int) {
	if len(os.Args) < 3 || strings.Contains(os.Args[2], "help") ||
		(os.Args[2] != "upload" && os.Args[2] != "list") {
		printBinaryUsage()
	}

	err := admin.TryConnect()
	if err != nil {
		return fmt.Errorf("Can't connect, %w", err), 0
	}

	switch os.Args[2] {
	case "upload":
		fs := flag.NewFlagSet("binary upload", flag.ExitOnError)
		name := fs.String("name", "", "name of the binary (epull, eserved, ...)")
		arch := fs.String("arch", "", "arch the binary was built for (gcc -dumpmachine)")
		path := fs.String("file", "", "path to the binary to upload")
		fs.Parse(os.Args[3:])
		if *name == "" || *arch == "" || *path == "" {
			return fmt.Errorf("-name, -arch and -file flags are required"), 2
		}
		if err := admin.PostUploadBinary(*name, *arch, *path); err != nil {
			return err, 1
		}
		fmt.Printf("uploaded %s for %s\n", *name, *arch)
	case "list":
		list, err := admin.PostListBinaries()
		if err != nil {
			return err, 1
		}
		if len(list.Binaries) == 0 {
			fmt.Println("no binaries")
			return nil, 0
		}
		fmt.Printf("%-12s %-24s %-16s %s\n", "NAME", "ARCH", "SIZE", "SHA256")
		for _, binary := range list.Binaries {
			fmt.Printf("%-12s %-24s %-16d %s\n", binary.Name, binary.Arch, binary.Size, binary.SHA256)
		}
	}

	return nil, 0
}

func main() {
	if len(os.Args) < 2 || strings.Contains(os.Args[1], "help") ||
		(os.Args[1] != "token" && os.Args[1] != "stage" && os.Args[1] != "machine" &&
			os.Args[1] != "job" && os.Args[1] != "build" && os.Args[1] != "flavor" && os.Args[1] != "binary") {
		printMainUsage()
	}

	switch os.Args[1] {
	case "token":
		if err, exitCode := parseTokenFlags(); err != nil {
			fmt.Fprintln(os.Stderr, "Something went wrong,", err)
			os.Exit(exitCode)
		}
	case "stage":
		if err, exitCode := parseStageFlags(); err != nil {
			fmt.Fprintln(os.Stderr, "Something went wrong,", err)
			os.Exit(exitCode)
		}
	case "machine":
		if err, exitCode := parseMachineFlags(); err != nil {
			fmt.Fprintln(os.Stderr, "Something went wrong,", err)
			os.Exit(exitCode)
		}
	case "job":
		if err, exitCode := parseJobFlags(); err != nil {
			fmt.Fprintln(os.Stderr, "Something went wrong,", err)
			os.Exit(exitCode)
		}
	case "build":
		if err, exitCode := parseBuildFlags(); err != nil {
			fmt.Fprintln(os.Stderr, "Something went wrong,", err)
			os.Exit(exitCode)
		}
	case "flavor":
		if err, exitCode := parseFlavorFlags(); err != nil {
			fmt.Fprintln(os.Stderr, "Something went wrong,", err)
			os.Exit(exitCode)
		}
	case "binary":
		if err, exitCode := parseBinaryFlags(); err != nil {
			fmt.Fprintln(os.Stderr, "Something went wrong,", err)
			os.Exit(exitCode)
		}
	}
}
