package tasks

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/adnsv/btr/codegen"
)

func init() {
	image.RegisterFormat("png", "png", png.Decode, png.DecodeConfig)
	image.RegisterFormat("jpeg", "jpeg", jpeg.Decode, jpeg.DecodeConfig)
}

func RunImgPackCPP(task *Task, config *Config) error {
	var err error
	targets := task.Targets
	if len(targets) != 2 {
		return errors.New("missing or invalid targets\nplease specify two targets \"targets\": [\"filepath.hpp\", \"filepath.cpp\"] in the task description")
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
	sources := task.GetSources()
	if len(sources) == 0 {
		return errors.New("missing sources paths\nspecify \"source\": \"path\": \"path\" or \"sources\": [\"path\",...] in the task description")
	}
	filepaths, err := ObtainFilePaths(config.BaseDir, sources)
	if err != nil {
		return err
	}

	namespace := task.Codegen.Namespace

	hpp := codegen.NewBuffer(hpath, config.Codegen)
	cpp := codegen.NewBuffer(cpath, config.Codegen)
	hpp.WriteLines(config.Codegen.TopMatter.Lines("hpp")...)
	hpp.WriteLines(task.Codegen.TopMatter.Lines("hpp")...)
	cpp.WriteLines(config.Codegen.TopMatter.Lines("cpp")...)
	cpp.WriteLines(task.Codegen.TopMatter.Lines("cpp")...)

	hpp.BeginCppNamespace(namespace)
	cpp.BeginCppNamespace(namespace)

	for _, path := range filepaths {
		if config.Verbose {
			fmt.Printf("loading %q\n", path)
		}
		name := filepath.Base(path)
		name = name[:len(name)-len(filepath.Ext(path))]
		name = strings.ReplaceAll(name, "-", "_")

		binary, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		reader := bytes.NewReader(binary)
		img, frmt, err := image.Decode(reader)
		if err != nil {
			return err
		}

		bounds := img.Bounds()
		size := bounds.Size()

		switch task.Format {
		case "prgba":
			{
				conv := image.NewRGBA(bounds)
				cm := conv.ColorModel()
				for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
					for x := bounds.Min.X; x < bounds.Max.X; x++ {
						s := img.At(x, y)
						conv.Set(x, y, cm.Convert(s))
					}
				}
				binary = []byte(conv.Pix)
				frmt = "prgba"
			}
		case "nrgba":
			{
				conv := image.NewNRGBA(bounds)
				cm := conv.ColorModel()
				for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
					for x := bounds.Min.X; x < bounds.Max.X; x++ {
						s := img.At(x, y)
						conv.Set(x, y, cm.Convert(s))
					}
				}
				binary = []byte(conv.Pix)
				frmt = "nrgba"
			}
		}

		hpp.Printf("// %s bitmap resource\n", name)
		hpp.Printf("// obtained from %s\n", filepath.Base(path))
		hpp.Printf("namespace %s {\n", name)
		hpp.Printf("\tconstexpr size_t width = %d;\n", size.X)
		hpp.Printf("\tconstexpr size_t height = %d;\n", size.Y)
		hpp.Printf("\tconstexpr char const* fmt = %q;\n", frmt)
		hpp.Printf("\textern const std::array<unsigned char const, %d> data;\n", len(binary))
		hpp.Print("}\n\n")

		cpp.Printf("namespace %s {\n", name)
		cpp.Printf("\tconst std::array<unsigned char const, %d> data = {", len(binary))
		s := ""
		for i, b := range binary {
			if i%32 == 0 {
				s += "\n\t"
			}
			s += fmt.Sprintf("0x%.2x,", b)
		}
		cpp.Print(s[:len(s)-1])
		cpp.Print("};\n")
		cpp.Print("}\n\n")
	}

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
