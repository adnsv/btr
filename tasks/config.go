package tasks

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"github.com/adnsv/btr/codegen"
	"gopkg.in/yaml.v3"
)

type Config struct {
	BaseDir string          `json:"-" yaml:"-"`
	Verbose bool            `json:"-" yaml:"-"`
	Version string          `json:"version" yaml:"version"`
	Codegen *codegen.Config `json:"codegen" yaml:"codegen"`
	Tasks   []*Task         `json:"tasks" yaml:"tasks"`
}

func LoadConfig(fn string) (*Config, error) {
	buf, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	config := &Config{}
	ext := strings.ToLower(filepath.Ext(fn))
	if ext == ".yaml" || ext == ".yml" {
		err = yaml.Unmarshal(buf, &config)
	} else {
		err = json.Unmarshal(buf, &config)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %q:\n%s",
			fn, jsonErrDetail(string(buf), err))
	}
	config.BaseDir, err = filepath.Abs(filepath.Dir(fn))
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}
	return config, nil
}

func (c *Config) Run() error {
	if len(c.Tasks) == 0 {
		return fmt.Errorf("No tasks specified")
	}
	for i, t := range c.Tasks {
		s := ""
		if t.Label != "" {
			s = fmt.Sprintf(": '%s'", t.Label)
		}
		fmt.Printf("Task %d of %d%s\n", i+1, len(c.Tasks), s)
		err := c.RunTask(t)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) RunTask(t *Task) error {
	if t.Type == "" {
		log.Printf("missing 'type' field\n")
		return nil
	}
	if c.Verbose {
		fmt.Printf("task type: %s\n", t.Type)
	}

	switch t.Type {
	case "svgfont.make":
		return RunSVGFontMake(t, c)
	case "svgfont.hpp":
		return RunSVGFontHPP(t, c)
	case "svgfont.ttf":
		return RunSVGFontTTF(t, c)
	case "binpack.c++":
		return RunBinPackCPP(t, c)
	case "imgpack.c++":
		return RunImgPackCPP(t, c)
	case "imgpack.c++.types":
		return RunImgPackCPPTypes(t, c)
	case "dir.make":
		return RunDir(t, c, "make")
	case "dir.clean":
		return RunDir(t, c, "clean")
	case "icon.win32":
		return RunIcon(t, c, "win32")
	default:
		log.Printf("unsupported task type '%s'", t.Type)
	}

	return nil
}
