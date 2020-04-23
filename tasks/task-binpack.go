package tasks

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/adnsv/btr/codegen"
)

func RunBinPackCPP(task *Task, config *Config) error {
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
	hpath = filepath.Clean(hpath)
	cpath = filepath.Clean(cpath)

	sources := task.GetSources()
	if len(sources) == 0 {
		return errors.New("missing source paths\nspecify \"source\": \"path\" or \"sources\": [\"path\",...] in the task description")
	}
	filepaths, err := AbsExistingPaths(config.BaseDir, sources)
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

		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		hpp.Printf("extern const std::array<unsigned char, %d> %s;\n",
			len(data), name)

		cpp.Printf("const std::array<unsigned char, %d> %s = {",
			len(data), name)
		s := ""
		for i, b := range data {
			if i%32 == 0 {
				s += "\n\t"
			}
			s += fmt.Sprintf("0x%.2x,", b)
		}
		cpp.Print(s[:len(s)-1])
		cpp.Print("};\n")
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
