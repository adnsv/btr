package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/adnsv/btr/codegen"

	"github.com/adnsv/btr/tasks"
	cli "github.com/jawher/mow.cli"
)

func main() {
	app := cli.App("rpk", "Resource packer utility")
	app.Spec = "[-v] FILENAME"
	fn := app.StringArg("FILENAME", "", "A JSON file with packer steps")
	verbose := app.BoolOpt("v verbose", false, "Show verbose output")
	app.Action = func() {
		cwd, _ := os.Getwd()

		absfn, _ := filepath.Abs(*fn)
		fmt.Printf("Running config: %s\n", absfn)
		if *verbose {
			fmt.Printf("CWD: %s\n", cwd)
		}

		config, err := tasks.LoadConfig(absfn)
		if err != nil {
			log.Fatal(err)
		}
		config.Verbose = *verbose
		if *verbose {
			fmt.Printf("Configuration loaded\n")
		}
		if config != nil {
			if config.Codegen == nil {
				config.Codegen = &codegen.Config{}
			}
			config.Codegen.OnBeforeWrite = func(path string) {
				fmt.Printf("writing %q", path)
			}
			config.Codegen.OnWriteSucceded = func(path string) {
				fmt.Print(" ... DONE\n")
			}
			config.Codegen.OnWriteFailed = func(path string, err error) {
				fmt.Print(" ... FAILED\n")
			}
			err = config.Run()
			if err != nil {
				log.Fatal(err)
			}
		}
		fmt.Printf("Mission Accomplished\n")
	}
	app.Run(os.Args)
}
