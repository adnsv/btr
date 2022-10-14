package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

var app_ver string = ""

func show_app_version() {
	v, ok := debug.ReadBuildInfo()
	if ok && v.Main.Version != "(devel)" {
		// installed with go install
		fmt.Println(v.Main.Version)
	} else if app_ver != "" {
		// built with ld-flags
		fmt.Println(app_ver)
	} else {
		fmt.Fprintln(os.Stderr, "version info is not available for this build")
		os.Exit(1)
	}
}
