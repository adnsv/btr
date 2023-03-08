package tasks

import (
	"log"
	"path/filepath"
)

// Task contains a description and all the parameters required for execution of
// a task
type Task struct {
	Label   string   `yaml:"label,omitempty"`
	Type    string   `yaml:"type,omitempty"`
	Source  string   `yaml:"source,omitempty"`
	Sources []string `yaml:"sources,omitempty"`
	Format  string   `yaml:"format,omitempty"`

	TtfTarget *TtfTarget `yaml:"ttf-target,omitempty"`
	SvgTarget *SvgTarget `yaml:"svg-target,omitempty"`
	HppTarget *HppTarget `yaml:"hpp-target,omitempty"`
	CppTarget *CppTarget `yaml:"cpp-target,omitempty"`
}

type TtfTarget struct {
	File string `yaml:"file"`
}

type SvgTarget struct {
	File           string `yaml:"file"`
	FirstCodePoint string `yaml:"first-codepoint"`
	Height         *int   `yaml:"height"`
	Descent        *int   `yaml:"descent"`
	Family         string `yaml:"family"`
}

type HppTarget struct {
	File    string `yaml:"file"`
	Entry   string `yaml:"entry"`
	Content string `yaml:"content"`
}

type CppTarget struct {
	File    string `yaml:"file"`
	Entry   string `yaml:"entry"`
	Content string `yaml:"content"`
}

// GetSources combines Source and Sources into a single list
func (t *Task) GetSources() []string {
	ret := []string{}
	if len(t.Source) > 0 {
		ret = append(ret, t.Source)
	}
	ret = append(ret, t.Sources...)
	return ret
}

// AbsExistingPaths gets all the actual filepaths from sources, processes
// wildcards and expands all paths relative to basedir
// returns paths only for existing filesystem entries
func (prj *Project) AbsExistingPaths(paths []string) []string {
	var err error
	ret := []string{}
	for _, it := range paths {
		path := it
		if !filepath.IsAbs(path) {
			path, err = filepath.Abs(filepath.Join(prj.BaseDir, it))
			if err != nil {
				log.Fatal(err)
			}
		}
		path = filepath.Clean(path)
		matches, err := filepath.Glob(path)
		if err != nil {
			log.Fatal(err)
		}
		for _, fn := range matches {
			ret = append(ret, filepath.Clean(fn))
		}
	}
	return ret
}

// AbsPaths converts paths to absolute paths
// non-absolute paths are expanded relative to basedir
func (prj *Project) AbsPaths(paths []string) []string {
	var err error
	ret := []string{}
	for _, it := range paths {
		path := it
		if !filepath.IsAbs(path) {
			path, err = filepath.Abs(filepath.Join(prj.BaseDir, it))
			if err != nil {
				log.Fatal(err)
			}
		}
		path = filepath.Clean(path)
		if len(path) > 0 {
			ret = append(ret, path)
		}
	}
	return ret
}

func (prj *Project) AbsPath(fn string) string {
	if filepath.IsAbs(fn) {
		return fn
	}
	fn, err := filepath.Abs(filepath.Join(prj.BaseDir, fn))
	if err != nil {
		log.Fatal(err)
	}
	return fn
}
