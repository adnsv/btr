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
	app := cli.App("btr", "Resource packer utility")
	app.Spec = "[--version] [--verbose] FILENAME"

	verbose := false
	showver := false

	fn := app.StringArg("FILENAME", "", "A JSON file that describes what needs to be done")
	app.BoolOptPtr(&verbose, "verbose", false, "Show verbose output")
	app.BoolOptPtr(&showver, "version", false, "Display version number")

	app.Action = func() {
		cwd, _ := os.Getwd()

		if showver {
			show_app_version()
			return
		}

		absfn, _ := filepath.Abs(*fn)
		fmt.Printf("Running config: %s\n", absfn)
		if verbose {
			fmt.Printf("CWD: %s\n", cwd)
		}

		config, err := tasks.LoadConfig(absfn)
		if err != nil {
			log.Fatal(err)
		}
		config.Verbose = verbose
		if verbose {
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
		fmt.Print("\nmission accomplished\n")
	}

	app.Run(os.Args)
}
