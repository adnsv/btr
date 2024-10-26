package tasks

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/adnsv/vgr-tools/vgr"
)

type VGConvertTask struct{}

func (VGConvertTask) Run(prj *Project, fields map[string]any) error {
	sources := []string{}
	var err error

	if v, ok := fields["source"]; ok {
		sources, err = prj.GetStrings(v)
		if err != nil {
			return fmt.Errorf("source field: %w", err)
		}
		if len(sources) == 0 {
			return fmt.Errorf("source field: must contain one or more filenames")
		}
	} else {
		return fmt.Errorf("missing field: source")
	}

	source_fns, err := prj.AbsExistingPaths(sources)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	} else if len(source_fns) == 0 {
		return fmt.Errorf("no sources found")
	}

	inputs := []*vgr.VG{}
	for _, fn := range source_fns {
		vg, err := vgr.ImportSVGFile(fn)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", fn, err)
		}
		inputs = append(inputs, vg)
	}

	dst, err := FetchCppTargetFields(prj, fields)
	if err != nil {
		return err
	}

	hpp, cpp := dst.MakeWriters()

	dst.PutFileHeader(hpp, cpp)

	fmt.Fprintf(hpp, "#include <array>\n\n")
	fmt.Fprintf(cpp, "#include %q\n\n", cpp.RelPathTo(hpp))

	hpp.StartNamespace()
	cpp.StartNamespace()

	for _, vg := range inputs {
		writeVG(hpp, cpp, vg)
	}

	hpp.DoneNamespace()
	cpp.DoneNamespace()

	err = hpp.WriteOutFile()
	if err != nil {
		return err
	}
	return cpp.WriteOutFile()
}

func writeVG(hpp, cpp io.Writer, src *vgr.VG) {
	buf := vgr.Pack(src)

	ident := MakeCPPIdentStr(strings.ToLower(RemoveExtension(filepath.Base(src.Filename))))

	bytestr := bytesToHexWrappedIndented(buf)

	fmt.Fprintf(hpp, "extern const std::array<unsigned char, %d> %s;\n\n",
		len(buf), ident)
	fmt.Fprintf(cpp, "const std::array<unsigned char, %d> %s = {\n%s\n};\n\n",
		len(buf), ident, bytestr)
}
