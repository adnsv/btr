package tasks

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/adnsv/vgr-tools/vgr"
)

func RunVGConvertTask(prj *Project, fields map[string]any) error {
	sources := []string{}
	hpp_fn := ""
	cpp_fn := ""
	namespace := ""

	var err error

	for k, v := range fields {
		switch k {

		case "source":
			sources, err = prj.GetStrings(v)
			if err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}

		case "hpp-target":
			if s, ok := v.(string); ok && s != "" {
				hpp_fn, err = prj.AbsPath(s)
				if err != nil {
					return fmt.Errorf("%s: %w", k, err)
				}
			} else {
				return fmt.Errorf("%s: must be a non-empty string", k)
			}

		case "cpp-target":
			if s, ok := v.(string); ok && s != "" {
				cpp_fn, err = prj.AbsPath(s)
				if err != nil {
					return fmt.Errorf("%s: %w", k, err)
				}
			} else {
				return fmt.Errorf("%s: must be a non-empty string", k)
			}

		case "namespace":
			if s, ok := v.(string); ok && s != "" {
				namespace = s
			} else {
				return fmt.Errorf("%s: must be a non-empty string", k)
			}
		}
	}

	if hpp_fn == "" {
		return fmt.Errorf("missing field: hpp-target")
	}

	if cpp_fn == "" {
		return fmt.Errorf("missing field: cpp-target")
	}

	if len(sources) == 0 {
		return fmt.Errorf("missing field: source")
	}

	source_fns, err := prj.AbsExistingPaths(sources)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	} else if len(source_fns) == 0 {
		return fmt.Errorf("no sources found")
	}

	hpp_buf := bytes.Buffer{}
	hpp_out := tabwriter.NewWriter(&hpp_buf, 0, 4, 4, ' ', 0)

	cpp_buf := bytes.Buffer{}
	cpp_out := tabwriter.NewWriter(&cpp_buf, 0, 4, 4, ' ', 0)

	inputs := []*vgr.VG{}
	for _, fn := range source_fns {
		vg, err := vgr.ImportSVGFile(fn)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", fn, err)
		}
		inputs = append(inputs, vg)
	}

	rel_name, _ := filepath.Rel(filepath.Dir(cpp_fn), hpp_fn)

	fmt.Fprintf(hpp_out, "#pragma once\n\n")
	fmt.Fprintf(hpp_out, "#include <array>\n\n")
	fmt.Fprintf(cpp_out, "#include %q\n\n", rel_name)

	if namespace != "" {
		fmt.Fprintf(hpp_out, "namespace %s {\n\n", namespace)
		fmt.Fprintf(cpp_out, "namespace %s {\n\n", namespace)
	}

	for _, vg := range inputs {
		writeVG(hpp_out, cpp_out, vg)
	}

	if namespace != "" {
		fmt.Fprintf(hpp_out, "} // namespace %s\n", namespace)
		fmt.Fprintf(cpp_out, "} // namespace %s\n", namespace)
	}

	hpp_out.Flush()
	cpp_out.Flush()

	fmt.Printf("- writing %s ... ", hpp_fn)
	err = os.WriteFile(hpp_fn, hpp_buf.Bytes(), 0666)
	if err == nil {
		fmt.Printf("SUCCEEDED\n")
	} else {
		fmt.Printf("FAILED\n")
	}

	fmt.Printf("- writing %s ... ", cpp_fn)
	err = os.WriteFile(cpp_fn, cpp_buf.Bytes(), 0666)
	if err == nil {
		fmt.Printf("SUCCEEDED\n")
	} else {
		fmt.Printf("FAILED\n")
	}

	return nil
}

func writeVG(hpp, cpp io.Writer, src *vgr.VG) {
	buf := vgr.Pack(src)

	ident := MakeCPPIdentStr(strings.ToLower(RemoveExtension(filepath.Base(src.Filename))))

	bb := []string{}
	for _, v := range buf {
		bb = append(bb, fmt.Sprintf("0x%.2x", v))
	}

	fmt.Fprintf(hpp, "extern const std::array<unsigned char, %d> %s;\n\n",
		len(buf), ident)
	fmt.Fprintf(cpp, "const std::array<unsigned char, %d> %s = {\n%s\n};\n\n",
		len(buf), ident, CommaWrap(bb, "\t", 100))
}
