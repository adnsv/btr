package tasks

import (
	"path/filepath"

	"github.com/adnsv/btr/codegen"
)

// Task contains a description and all the parameters required for execution of
// a task
type Task struct {
	Label   string      `json:"label,omitempty" yaml:"label,omitempty"`
	Type    string      `json:"type,omitempty" yaml:"type,omitempty"`
	Source  string      `json:"source,omitempty" yaml:"source,omitempty"`
	Sources []string    `json:"sources,omitempty" yaml:"sources,omitempty"`
	Target  string      `json:"target,omitempty" yaml:"target,omitempty"`
	Targets []string    `json:"targets,omitempty" yaml:"targets,omitempty"`
	Font    *FontConfig `json:"font,omitempty" yaml:"font,omitempty"`
	Codegen TaskCodegen `json:"codegen,omitempty" yaml:"codegen,omitempty"`
	Format  string      `json:"format,omitempty" yaml:"format,omitempty"`
}

type TaskCodegen struct {
	Namespace    string         `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	TypeName     *string        `json:"typename,omitempty" yaml:"typename,omitempty"`
	ValuePrefix  string         `json:"value.prefix,omitempty" yaml:"value-prefix,omitempty"`
	ValuePostfix string         `json:"value.postfix,omitempty" yaml:"value-postfix,omitempty"`
	IdentPrefix  string         `json:"ident.prefix,omitempty" yaml:"ident-prefix,omitempty"`
	IdentPostfix string         `json:"ident.postfix,omitempty" yaml:"ident-postfix,omitempty"`
	TopMatter    codegen.Matter `json:"top-matter,omitempty" yaml:"top-matter,omitempty"`
	BottomMatter codegen.Matter `json:"bottom-matter,omitempty" yaml:"bottom-matter,omitempty"`
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
