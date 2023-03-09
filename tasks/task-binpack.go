package tasks

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"golang.org/x/exp/maps"
)

func RunBinpackTask(prj *Project, fields map[string]any) error {
	sources := []string{}
	targets := []*Target{}
	var err error
	for k, v := range fields {
		switch k {
		case "source":
			sources, err = prj.GetStrings(v)
			if err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}

		case "target":
			targets, err = prj.GetTargets(v)
			if err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
			if len(targets) == 0 {
				return fmt.Errorf("%s: must not be empty", k)
			}
		}
	}

	if len(sources) == 0 {
		return fmt.Errorf("missing field: source")
	}
	if len(targets) == 0 {
		return fmt.Errorf("missing field: target")
	}

	source_fns, err := prj.AbsExistingPaths(sources)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	} else if len(source_fns) == 0 {
		return fmt.Errorf("no sources found")
	}

	type blobInfo struct {
		filename  string
		ident_cpp string
		data      []byte
		bytestr   string
	}

	blobs := []*blobInfo{}
	for _, source_fn := range source_fns {
		if prj.Verbose {
			fmt.Printf("- reading: %s\n", source_fn)
		}
		data, err := os.ReadFile(source_fn)
		if err != nil {
			return err
		}

		filename := filepath.Base(source_fn)
		ident_cpp := strings.ToLower(MakeCPPIdentStr(strings.ToLower(filename)))

		bytestr := "    "
		for i, b := range data {
			if i > 0 && i%32 == 0 {
				bytestr += "\n    "
			}
			bytestr += fmt.Sprintf("0x%.2x,", b)
		}
		blobs = append(blobs, &blobInfo{filename: filename, ident_cpp: ident_cpp, data: data, bytestr: bytestr})
	}

	for _, target := range targets {
		entries := []string{}

		for _, blob := range blobs {
			entry_vars := maps.Clone(prj.Vars)
			entry_vars["byte-count"] = fmt.Sprintf("%d", len(blob.data))
			entry_vars["byte-content"] = blob.bytestr
			entry_vars["filename"] = blob.filename
			entry_vars["ident-cpp"] = blob.ident_cpp
			entry, err := ExpandVariables(target.Entry, entry_vars)
			if err != nil {
				return err
			}
			entries = append(entries, entry)
		}

		file_vars := maps.Clone(prj.Vars)
		file_vars["entries"] = strings.Join(entries, "\n\n")
		content, err := ExpandVariables(target.Content, file_vars)
		if err != nil {
			return err
		}

		buf := bytes.Buffer{}
		out := tabwriter.NewWriter(&buf, 0, 4, 1, ' ', 0)
		fmt.Fprint(out, content)
		out.Flush()

		fmt.Printf("- writing %s ... ", target.File)
		err = os.WriteFile(target.File, buf.Bytes(), 0666)
		if err == nil {
			fmt.Printf("SUCCEEDED\n")
		} else {
			fmt.Printf("FAILED\n")
		}
	}

	return nil
}
