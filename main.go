package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/adnsv/btr/tasks"
)

func printHelp(w io.Writer) {
	fmt.Fprint(w, `btr - a build-task-runner utility (https://github.com/adnsv/btr)

usage: btr [options] <filename>

<filename>      A yaml file that describes what needs to be done
                (defaults to build-tasks.yaml in CWD).

options:
    --version   Display application version and exit.
	--verbose   Provide detailed information when running tasks.
`)
}

func main() {
	verbose := false
	args := []string{}

	for _, a := range os.Args[1:] {
		if a == "" {
			continue
		} else if a[0] == '-' {
			if a == "--version" {
				fmt.Println(app_version())
				os.Exit(0)
			} else if a == "-h" || a == "--help" {
				printHelp(os.Stdout)
			} else if a == "-v" || a == "--verbose" {
				verbose = true
			} else {
				fmt.Printf("warning: unsupported arg %s\n", a)
			}
		} else {
			args = append(args, a)
		}
	}

	proj_dir := ""
	proj_fn := ""
	var err error
	if len(args) == 0 {
		proj_dir, err = os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
	} else if len(args) > 1 {
		log.Fatal("invalid command line syntax: more than one argument provided")
	} else {
		stat, err := os.Stat(args[0])
		if err == nil && stat.IsDir() {
			proj_dir = args[0]
		} else {
			proj_fn = args[0]
		}
	}
	if proj_fn == "" {
		proj_fn = filepath.Join(proj_dir, "build-tasks.yaml")
		if _, err := os.Stat(proj_fn); os.IsNotExist(err) {
			proj_fn = filepath.Join(proj_dir, "build-tasks.yml")
			if _, err = os.Stat(proj_fn); os.IsNotExist(err) {
				log.Fatal("failed to load task descriptions\n" +
					"specify the path to the btr project file (e.g., build-tasks.yml)" +
					"or run btr from a directory that contains that file.")
			}
		}
	}

	proj_fn, err = filepath.Abs(proj_fn)
	if err != nil {
		log.Fatal(err)
	}
	if verbose {
		fmt.Printf("opening task descriptions from: %s\n", proj_fn)
	}

	prj, err := tasks.LoadProject(proj_fn)
	if err != nil {
		log.Fatal(err)
	}

	if verbose {
		fmt.Printf("loaded tasks: %d\n", len(prj.Tasks))
		fmt.Printf("running loaded tasks\n")
	}
	prj.Verbose = verbose
	err = prj.Run()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("\nmission accomplished\n")
}
