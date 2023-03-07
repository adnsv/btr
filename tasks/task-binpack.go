package tasks

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"golang.org/x/exp/maps"
)

func RunBinPackCPP(task *Task, config *Config) error {
	var err error
	targets := task.Targets
	if len(targets) != 2 {
		return errors.New("missing or invalid targets\nplease specify two targets \"targets\": [\"filepath.hpp\", \"filepath.cpp\"] in the task description")
	}
	sources := task.GetSources()
	if len(sources) == 0 {
		return errors.New("missing 'source' field")
	}
	filepaths, err := AbsExistingPaths(config.BaseDir, sources)
	if err != nil {
		return err
	}
	if task.HppTemplate == "" {
		return errors.New("missing 'hpp-template' field")
	}
	if task.CppTemplate == "" {
		return errors.New("missing 'cpp-template' field")
	}
	if task.HppEntryTemplate == "" {
		return errors.New("missing 'hpp-entry-template' field")
	}
	if task.CppEntryTemplate == "" {
		return errors.New("missing 'cpp-entry-template' field")
	}

	hpath := targets[0]
	cpath := targets[1]
	if !filepath.IsAbs(hpath) {
		hpath = filepath.Join(config.BaseDir, hpath)
		hpath, err = filepath.Abs(hpath)
		if err != nil {
			return err
		}
	}
	if !filepath.IsAbs(cpath) {
		cpath = filepath.Join(config.BaseDir, cpath)
		cpath, err = filepath.Abs(cpath)
		if err != nil {
			return err
		}
	}
	hpath = filepath.Clean(hpath)
	cpath = filepath.Clean(cpath)

	hpp_entries := []string{}
	cpp_entries := []string{}
	for _, path := range filepaths {
		if config.Verbose {
			fmt.Printf("loading %q\n", path)
		}

		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		bytestr := ""
		for i, b := range data {
			if i%32 == 0 {
				bytestr += "\n\t"
			}
			bytestr += fmt.Sprintf("0x%.2x,", b)
		}

		filename := filepath.Base(path)
		ident := filename[:len(filename)-len(filepath.Ext(path))]
		ident = strings.ReplaceAll(filename, "-", "_")

		entry_vars := map[string]string{
			"byte-count": fmt.Sprintf("%d", len(data)),
			"bytes":      bytestr,
			"filename":   filename,
			"ident":      ident,
		}

		hpp_entry, err := ExpandVariables(task.HppEntryTemplate, entry_vars)
		if err != nil {
			return fmt.Errorf("hpp-entry-template: %w", err)
		}
		cpp_entry, err := ExpandVariables(task.CppEntryTemplate, entry_vars)
		if err != nil {
			return fmt.Errorf("cpp-entry-template: %w", err)
		}
		hpp_entries = append(hpp_entries, hpp_entry)
		cpp_entries = append(cpp_entries, cpp_entry)
	}

	{
		vars := maps.Clone(config.Vars)
		vars["entries"] = strings.Join(hpp_entries, "\n\n")
		cont, err := ExpandVariables(task.HppTemplate, vars)
		if err != nil {
			return fmt.Errorf("hpp-template: %w", err)
		}
		buf := bytes.Buffer{}
		out := tabwriter.NewWriter(&buf, 0, 4, 1, ' ', 0)

	}

	cpp_buf := bytes.Buffer{}
	cpp_out := tabwriter.NewWriter(&cpp_buf, 0, 4, 1, ' ', 0)

	hpp.EndCppNamespace(namespace)
	hpp.WriteLines(task.Codegen.BottomMatter.Lines("hpp")...)
	hpp.WriteLines(config.Codegen.BottomMatter.Lines("hpp")...)
	cpp.EndCppNamespace(namespace)
	hpp.WriteLines(task.Codegen.BottomMatter.Lines("cpp")...)
	hpp.WriteLines(config.Codegen.BottomMatter.Lines("cpp")...)

	err = hpp.WriteOut()
	if err != nil {
		return err
	}
	return cpp.WriteOut()
}
