package tasks

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"golang.org/x/exp/maps"
)

func (prj *Project) RunBinPackCPP(task *Task) error {
	var err error
	src_fns := prj.AbsExistingPaths(task.GetSources())
	if len(sources) == 0 {
		return errors.New("missing source")
	}

	if task.HppTarget == nil {
		return errors.New("missing 'hpp-target")
	}
	if task.HppTarget.File == "" {
		return errors.New("missing 'hpp-target.file")
	}
	if task.HppTarget.Entry == "" {
		return errors.New("missing 'hpp-target.entry")
	}
	if task.HppTarget.Content == "" {
		return errors.New("missing 'hpp-target.content")
	}
	if task.CppTarget == nil {
		return errors.New("missing 'cpp-target")
	}
	if task.CppTarget.File == "" {
		return errors.New("missing 'cpp-target.file")
	}
	if task.CppTarget.Entry == "" {
		return errors.New("missing 'cpp-target.entry")
	}
	if task.CppTarget.Content == "" {
		return errors.New("missing 'cpp-target.content")
	}

	hpp_fn := prj.AbsPath(task.HppTarget.File)
	cpp_fn := prj.AbsPath(task.CppTarget.File)

	hpp_entries := []string{}
	cpp_entries := []string{}
	for _, path := range src_fns {
		if prj.Verbose {
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

		hpp_entry, err := ExpandVariables(task.HppTarget.Entry, entry_vars)
		if err != nil {
			return fmt.Errorf("hpp-target.entry: %w", err)
		}
		cpp_entry, err := ExpandVariables(task.CppTarget.Entry, entry_vars)
		if err != nil {
			return fmt.Errorf("cpp-target.entry: %w", err)
		}
		hpp_entries = append(hpp_entries, hpp_entry)
		cpp_entries = append(cpp_entries, cpp_entry)
	}

	{
		vars := maps.Clone(prj.Vars)
		vars["entries"] = strings.Join(hpp_entries, "\n\n")
		cont, err := ExpandVariables(task.HppTarget.Content, vars)
		if err != nil {
			return fmt.Errorf("hpp-template: %w", err)
		}
		buf := bytes.Buffer{}
		out := tabwriter.NewWriter(&buf, 0, 4, 1, ' ', 0)
		fmt.Fprint(out, cont)
		fmt.Printf("writing %s ... ", hpp_fn)
		err = os.WriteFile(hpp_fn, buf.Bytes(), 0x666)
		if err == nil {
			fmt.Printf("SUCCEEDED\n")
		} else {
			fmt.Printf("FAILED\n")
			return err
		}

	}
	{
		vars := maps.Clone(prj.Vars)
		vars["entries"] = strings.Join(cpp_entries, "\n\n")
		cont, err := ExpandVariables(task.CppTarget.Content, vars)
		if err != nil {
			return fmt.Errorf("cpp-template: %w", err)
		}
		buf := bytes.Buffer{}
		out := tabwriter.NewWriter(&buf, 0, 4, 1, ' ', 0)
		fmt.Fprint(out, cont)
		fmt.Printf("writing %s ... ", cpp_fn)
		err = os.WriteFile(cpp_fn, buf.Bytes(), 0x666)
		if err == nil {
			fmt.Printf("SUCCEEDED\n")
		} else {
			fmt.Printf("FAILED\n")
			return err
		}
	}

	return err
}
