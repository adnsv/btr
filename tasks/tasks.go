package tasks

import (
	"path/filepath"
)

// Task contains a description and all the parameters required for execution of
// a task
type Task struct {
	Label   string      `yaml:"label,omitempty"`
	Type    string      `yaml:"type,omitempty"`
	Source  string      `yaml:"source,omitempty"`
	Sources []string    `yaml:"sources,omitempty"`
	Target  string      `yaml:"target,omitempty"`
	Targets []string    `yaml:"targets,omitempty"`
	Font    *FontConfig `yaml:"font,omitempty"`
	Format  string      `yaml:"format,omitempty"`

	HppTarget *HppTarget `yaml:"hpp-target"`
	CppTarget *CppTarget `yaml:"cpp-target"`
}

type FontConfig struct {
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

// GetTargets combines Source and Sources into a single list
func (t *Task) GetTargets() []string {
	ret := []string{}
	if len(t.Target) > 0 {
		ret = append(ret, t.Target)
	}
	for _, t := range t.Targets {
		if len(t) > 0 {
			ret = append(ret, t)
		}
	}
	return ret
}

// AbsExistingPaths gets all the actual filepaths from sources, processes
// wildcards and expands all paths relative to basedir
// returns paths only for existing filesystem entries
func AbsExistingPaths(basedir string, paths []string) ([]string, error) {
	var err error
	ret := []string{}
	for _, it := range paths {
		path := it
		if !filepath.IsAbs(path) {
			path, err = filepath.Abs(filepath.Join(basedir, it))
			if err != nil {
				return nil, err
			}
		}
		path = filepath.Clean(path)
		matches, err := filepath.Glob(path)
		if err != nil {
			return nil, err
		}
		for _, fn := range matches {
			ret = append(ret, filepath.Clean(fn))
		}
	}
	return ret, nil
}

// AbsPaths converts paths to absolute paths
// non-absolute paths are expanded relative to basedir
func AbsPaths(basedir string, paths []string) ([]string, error) {
	var err error
	ret := []string{}
	for _, it := range paths {
		path := it
		if !filepath.IsAbs(path) {
			path, err = filepath.Abs(filepath.Join(basedir, it))
			if err != nil {
				return nil, err
			}
		}
		path = filepath.Clean(path)
		if len(path) > 0 {
			ret = append(ret, path)
		}
	}
	return ret, nil
}
