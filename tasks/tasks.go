package tasks

import (
	"path/filepath"

	"github.com/adnsv/btr/codegen"
)

// Task contains a description and all the parameters required for execution of
// a task
type Task struct {
	Label   string      `json:"label"`
	Type    string      `json:"type"`
	Source  string      `json:"source"`
	Sources []string    `json:"sources"`
	Target  string      `json:"target"`
	Targets []string    `json:"targets"`
	Font    *FontConfig `json:"font"`
	Codegen TaskCodegen `json:"codegen"`
	Format  string      `json:"format"`
}

type TaskCodegen struct {
	Namespace    string         `json:"namespace"`
	TypeName     *string        `json:"typename"`
	ValuePrefix  string         `json:"value.prefix"`
	ValuePostfix string         `json:"value.postfix"`
	IdentPrefix  string         `json:"ident.prefix"`
	IdentPostfix string         `json:"ident.postfix"`
	TopMatter    codegen.Matter `json:"top-matter"`
	BottomMatter codegen.Matter `json:"bottom-matter"`
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