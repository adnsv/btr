package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/adnsv/btr/codegen"
	"gopkg.in/yaml.v3"

	"github.com/adnsv/btr/tasks"
	cli "github.com/jawher/mow.cli"
)

func main() {

	app := cli.App("btr", "Resource packer utility")
	app.Spec = "[--verbose] [--convert-to=<fn>] FILENAME"
	app.Version("version", app_version())

	verbose := false
	convert_to := ""

	fn := app.StringArg("FILENAME", "", "A YAML or JSON file that describes what needs to be done")
	app.BoolOptPtr(&verbose, "v verbose", false, "Show verbose output")
	app.StringOptPtr(&convert_to, "convert-to", "", "Convert configuration to another format")

	app.Action = func() {
		cwd, _ := os.Getwd()

		absfn, _ := filepath.Abs(*fn)
		fmt.Printf("Running config: %s\n", absfn)
		if verbose {
			fmt.Printf("CWD: %s\n", cwd)
		}

		config, err := tasks.LoadConfig(absfn)
		if err != nil {
			log.Fatal(err)
		}
		if convert_to != "" {
			ext := strings.ToLower(filepath.Ext(convert_to))
			switch ext {
			case ".yaml", ".yml":
				buf, _ := yaml.Marshal(*config)
				err = os.WriteFile(convert_to, buf, 0655)
				if err != nil {
					log.Fatal(err)
				}
			case ".json", ".jsn":
				buf, _ := json.MarshalIndent(config, "", "\t")
				err = os.WriteFile(convert_to, buf, 0655)
				if err != nil {
					log.Fatal(err)
				}
			}
			return
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
