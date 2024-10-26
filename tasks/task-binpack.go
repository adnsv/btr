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

type BinpackFileTask struct {
}

func (BinpackFileTask) Run(prj *Project, fields map[string]any) error {
	var source string
	var err error

	if v, ok := fields["source"]; ok {
		source, err = prj.GetString(v, true)
		if err != nil {
			return fmt.Errorf("source field: %w", err)
		}
		if len(source) == 0 {
			return fmt.Errorf("source field: must contain a filename")
		}
	} else {
		return fmt.Errorf("missing field: source")
	}

	var ident string
	if v, ok := fields["ident"]; ok {
		ident, err = prj.GetString(v, true)
		if err != nil {
			return fmt.Errorf("ident field: %w", err)
		}
	}

	var element_type string
	if v, ok := fields["element-type"]; ok {
		ident, err = prj.GetString(v, true)
		if err != nil {
			return fmt.Errorf("element-type field: %w", err)
		}
	}

	source_fn, err := prj.AbsPath(source)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	} else if len(source_fn) == 0 {
		return fmt.Errorf("missing source")
	}

	if ident == "" {
		// automatically generate ident from filename
		ident = strings.ToLower(MakeCPPIdentStr(strings.ToLower(filepath.Base(source_fn))))
	}

	if prj.Verbose {
		fmt.Printf("- reading: %s\n", source_fn)
	}
	data, err := os.ReadFile(source_fn)
	if err != nil {
		return err
	}
	bytestr := bytesToHexWrappedIndented(data)

	dst, err := FetchCppTargetFields(prj, fields)
	if err != nil {
		return err
	}

	hpp, cpp := dst.MakeWriters()

	hpp_includes := []string{}
	cpp_includes := []string{}

	hpp_includes = append(hpp_includes, "<array>")
	switch element_type {
	case "":
		element_type = "unsigned char"
	case "std::size_t":
		hpp_includes = append(hpp_includes, "<cstddef>")
	case "std::byte":
		hpp_includes = append(hpp_includes, "<cstddef>")
	}

	if element_type == "" {
		element_type = "unsigned char"
	}

	cpp_includes = append(cpp_includes, "\""+cpp.RelPathTo(hpp)+"\"")

	dst.PutFileHeader(hpp, cpp)

	for _, v := range hpp_includes {
		fmt.Fprintf(hpp, "#include %s\n", v)
	}
	fmt.Fprintf(hpp, "\n")

	for _, v := range cpp_includes {
		fmt.Fprintf(cpp, "#include %s\n", v)
	}
	fmt.Fprintf(cpp, "\n")

	hpp.StartNamespace()
	cpp.StartNamespace()

	fmt.Fprintf(hpp, "extern const std::array<%s, %d> %s;\n\n", element_type, len(data), ident)

	fmt.Fprintf(cpp, "const std::array<%s, %d> %s = {\n", element_type, len(data), ident)
	fmt.Fprint(cpp, bytestr)
	fmt.Fprintf(cpp, "};\n\n")

	hpp.DoneNamespace()
	cpp.DoneNamespace()

	err = hpp.WriteOutFile()
	if err != nil {
		return err
	}
	return cpp.WriteOutFile()
}

type BinpackTask struct {
}

func (BinpackTask) Run(prj *Project, fields map[string]any) error {
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

		bytestr := bytesToHexWrappedIndented(data)
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
