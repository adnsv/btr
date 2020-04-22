package tasks

import (
	"path/filepath"
)

// Task contains a description and all the parameters required for execution of
// a task
type Task struct {
	Label   string      `json:"label"`
	Type    string      `json:"type"`
	Source  string      `json:"source"`
	Sources []string    `json:"sources"`
	Target  string      `json:"target"`
	Font    *FontConfig `json:"font"`
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

// ObtainFilePaths gets all the actual filepaths from sources, processes
// wildcards and expands all paths relative to basedir
func ObtainFilePaths(basedir string, sources []string) ([]string, error) {
	ret := []string{}
	for _, src := range sources {
		if !filepath.IsAbs(src) {
			src = filepath.Join(basedir, src)
		}
		src, err := filepath.Abs(src)
		if err != nil {
			return nil, err
		}
		matches, err := filepath.Glob(src)
		if err != nil {
			return nil, err
		}
		for _, fn := range matches {
			ret = append(ret, fn)
		}
	}
	return ret, nil
}
