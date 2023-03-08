package tasks

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Project struct {
	BaseDir string            `yaml:"-"`
	Verbose bool              `yaml:"-"`
	Version string            `yaml:"version"`
	Vars    map[string]string `yaml:"vars"`
	Tasks   []*Task           `yaml:"tasks"`
}

func LoadProject(fn string) (*Project, error) {
	buf, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	prj := &Project{}
	ext := strings.ToLower(filepath.Ext(fn))
	if ext == ".yaml" || ext == ".yml" {
		err = yaml.Unmarshal(buf, &prj)
	} else if ext == ".json" {
		return nil, fmt.Errorf("json format is no longer supported")
	} else {
		return nil, fmt.Errorf("only files with .yaml and .yml extensions are supported")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %q:\n%s",
			fn, err)
	}
	prj.BaseDir, err = filepath.Abs(filepath.Dir(fn))
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}
	return prj, nil
}

func (prj *Project) Run() error {
	if len(prj.Tasks) == 0 {
		return fmt.Errorf("No tasks specified")
	}
	for i, t := range prj.Tasks {
		s := ""
		if t.Label != "" {
			s = fmt.Sprintf(": '%s'", t.Label)
		}
		fmt.Printf("Task %d of %d%s\n", i+1, len(prj.Tasks), s)
		err := prj.RunTask(t)
		if err != nil {
			return err
		}
	}
	return nil
}

func (prj *Project) RunTask(t *Task) error {
	if t.Type == "" {
		log.Printf("missing 'type' field\n")
		return nil
	}
	if prj.Verbose {
		fmt.Printf("task type: %s\n", t.Type)
	}

	switch t.Type {
	case "svgfont.make":
		return prj.ComposeSVGFilesIntoSVGFont(t)
	case "svgfont.hpp":
		return prj.CodeGenGlyphLookup(t)
	case "svgfont.ttf":
		return prj.ConvertSVGFontToTTF(t)
	case "binpack.c++":
		return RunBinPackCPP(t, prj)
	case "imgpack.c++":
		return RunImgPackCPP(t, prj)
	case "imgpack.c++.types":
		return RunImgPackCPPTypes(t, prj)
	case "dir.make":
		return RunDir(t, prj, "make")
	case "dir.clean":
		return RunDir(t, prj, "clean")
	case "icon.win32":
		return RunIcon(t, prj, "win32")
	default:
		log.Printf("unsupported task type '%s'", t.Type)
	}

	return nil
}
