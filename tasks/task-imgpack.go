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
	"sort"
	"strings"

	"github.com/adnsv/btr/codegen"
)

func init() {
	image.RegisterFormat("png", "png", png.Decode, png.DecodeConfig)
	image.RegisterFormat("jpeg", "jpeg", jpeg.Decode, jpeg.DecodeConfig)
}

type imageEntry struct {
	path   string
	name   string
	width  int
	height int
	frmt   string
	data   []byte
}

func loadImage(path string, name string, wantFormat string) (*imageEntry, error) {
	binary, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(binary)
	img, frmt, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	size := bounds.Size()

	switch wantFormat {
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
	ret := &imageEntry{}
	ret.path = path
	ret.name = name
	ret.width = size.X
	ret.height = size.Y
	ret.frmt = frmt
	ret.data = binary
	return ret, nil
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

	images := []*imageEntry{}

	nsprefix := namespace
	if len(nsprefix) > 0 {
		nsprefix += "::"
	}

	maxPathLength := 0
	for _, fn := range filepaths {
		if len(fn) > maxPathLength {
			maxPathLength = len(fn)
		}
	}

	for _, path := range filepaths {
		name := filepath.Base(path)
		name = name[:len(name)-len(filepath.Ext(path))]
		name = strings.ReplaceAll(name, "-", "_")

		n := maxPathLength - len(path) + 1
		if n < 1 {
			n = 1
		}
		if config.Verbose {
			fmt.Printf("loading %q%s-> %s%s\n", path, strings.Repeat(" ", n), nsprefix, name)
		}
		img, err := loadImage(path, name, task.Format)
		if err != nil {
			return err
		}
		images = append(images, img)
	}

	sort.SliceStable(images, func(i, j int) bool {
		return strings.Compare(images[i].name, images[j].name) < 0
	})

	typename := "image"
	if task.Codegen.TypeName != nil {
		typename = *task.Codegen.TypeName
	}

	for _, img := range images {
		hpp.Printf("// %s image resource\n", img.name)
		hpp.Printf("// %d x %d, %s, %d bytes\n", img.width, img.height, img.frmt, len(img.data))
		hpp.Printf("extern const %s %s;\n", typename, img.name)
	}

	cpp.Print("namespace { // hidden\n")
	cpp.Print("namespace image_data__ {\n")
	for _, img := range images {
		cpp.Printf("\n// %s\n", img.name)
		cpp.Printf("// width: %d; height: %d, fmt: %s\n", img.width, img.height, img.frmt)
		cpp.Printf("// size: %d bytes\n", len(img.data))
		cpp.Printf("const std::array<unsigned char const, %d> %s = {", len(img.data), img.name)
		s := ""
		for i, b := range img.data {
			if i%32 == 0 {
				s += "\n\t"
			}
			s += fmt.Sprintf("0x%.2x,", b)
		}
		cpp.Print(s[:len(s)-1])
		cpp.Print("};\n")
	}
	cpp.Print("\n} // image_data__ namespace \n\n")

	cpp.Printf("const std::array<%s const, %d> image_catalog__ = {", typename, len(images))
	for _, img := range images {
		cpp.Printf("\n\t%s{%q, %q, %d, %d, {image_data__::%s.data(), image_data__::%s.size()} },",
			typename, img.name, img.frmt, img.width, img.height, img.name, img.name)
	}
	cpp.Printf("\n};\n\n")
	cpp.Print("} // hidden namespace\n\n")

	for i, img := range images {
		cpp.Printf("const %s const& %s = image_catalog__[%d];\n",
			typename, img.name, i)
	}

	cpp.Printf("\nauto find_image(std::string_view name) -> %s const* {\n", typename)
	cpp.Print("\tfor (auto const& it : image_catalog__)\n")
	cpp.Print("\t\tif (it.name == name) return &it;\n")
	cpp.Print("\treturn nullptr;\n")
	cpp.Print("}\n")

	hpp.EndCppNamespace(namespace)
	hpp.WriteLines(task.Codegen.BottomMatter.Lines("hpp")...)
	hpp.WriteLines(config.Codegen.BottomMatter.Lines("hpp")...)
	cpp.EndCppNamespace(namespace)
	cpp.WriteLines(task.Codegen.BottomMatter.Lines("cpp")...)
	cpp.WriteLines(config.Codegen.BottomMatter.Lines("cpp")...)

	err = hpp.WriteOut()
	if err != nil {
		return err
	}
	return cpp.WriteOut()

}

func RunImgPackCPPTypes(task *Task, config *Config) error {
	var err error
	dstpath := task.Target
	if len(dstpath) == 0 {
		return errors.New("missing target path\nplease specify \"target\": \"filepath\" in the task description")
	}
	if !filepath.IsAbs(dstpath) {
		dstpath = filepath.Join(config.BaseDir, dstpath)
		dstpath, err = filepath.Abs(dstpath)
		if err != nil {
			return err
		}
	}

	hpp := codegen.NewBuffer(dstpath, config.Codegen)
	hpp.WriteLines(config.Codegen.TopMatter.Lines("hpp")...)
	hpp.WriteLines(task.Codegen.TopMatter.Lines("hpp")...)

	hpp.BeginCppNamespace(task.Codegen.Namespace)

	typename := "image"
	if task.Codegen.TypeName != nil {
		typename = *task.Codegen.TypeName
	}

	hpp.Printf(`struct %s {
	std::string_view name;
	std::string_view format;
	std::size_t width;
	std::size_t height;
	std::basic_string_view<unsigned char const> bytes;
};
`, typename)

	hpp.EndCppNamespace(task.Codegen.Namespace)
	hpp.WriteLines(task.Codegen.BottomMatter.Lines("hpp")...)
	hpp.WriteLines(config.Codegen.BottomMatter.Lines("hpp")...)

	return hpp.WriteOut()
}
