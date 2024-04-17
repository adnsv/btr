package tasks

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

// Project contains global vars and tasks.
type Project struct {
	BaseDir string            `yaml:"-"`
	Verbose bool              `yaml:"-"`
	Version string            `yaml:"version"`
	Vars    map[string]string `yaml:"vars"`
	Tasks   []*Task           `yaml:"tasks"`
}

// Task
type Task struct {
	Name   string         `yaml:"name,omitempty"`
	Type   string         `yaml:"type,omitempty"`
	Fields map[string]any `yaml:",inline"`
}

func LoadProject(fn string) (*Project, error) {
	buf, err := os.ReadFile(fn)
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

func (prj *Project) ValidateVersion(appver string) error {
	if appver == "(devel)" || appver == "#UNAVAILABLE" {
		if prj.Verbose {
			fmt.Printf("skipping version check: running devel build")
		}
		return nil
	}

	if prj.Version == "" {
		fmt.Printf("WARNING: skipping version check: missing version field in the project file")
		return nil
	}
	projsemver, err := semver.ParseTolerant(prj.Version)
	if err != nil {
		return fmt.Errorf("version synax in '%s': %w", prj.Version, err)
	}

	appsemver, err := semver.ParseTolerant(appver)
	if err != nil {
		return fmt.Errorf("version check: failed to parse app version '%s'", appver)
	}

	if projsemver.Compare(appsemver) > 0 {
		fmt.Printf("btr version >= %s is required to execute these tasks\n", projsemver)
		fmt.Printf("you are using version %s\n", appsemver)
		fmt.Printf("please update btr, see https://github.com/adnsv/btr for details\n")
		return fmt.Errorf("version check: unsupported version")
	}

	return nil
}

func (prj *Project) Run() error {
	if len(prj.Tasks) == 0 {
		return fmt.Errorf("no tasks specified")
	}
	for i, t := range prj.Tasks {
		s := ""
		if t.Name != "" {
			s = fmt.Sprintf(": '%s'", t.Name)
		}
		fmt.Printf("Task %d of %d%s\n", i+1, len(prj.Tasks), s)
		err := prj.RunTask(t)
		if err != nil {
			s := fmt.Sprintf("task[%d]", i)
			if t.Name != "" {
				s = fmt.Sprintf("%s, '%s'", s, t.Name)
			}
			return fmt.Errorf("%s: %w", s, err)
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
		fmt.Printf("- type: %s\n", t.Type)
	}

	var err error
	switch t.Type {
	case "dir":
		err = RunDirTask(prj, t.Fields)
	case "file":
		err = RunFileTask(prj, t.Fields)
	case "binpack":
		err = RunBinpackTask(prj, t.Fields)
	case "svgfont":
		err = RunSVGFontTask(prj, t.Fields)
	case "ttf":
		err = RunTTFTask(prj, t.Fields)
	case "glyph-names":
		err = RunGlyphNamesTask(prj, t.Fields)
	case "embed-icon":
		err = RunEmbedIconTask(prj, t.Fields)
	case "win32-icon":
		err = RunWin32IconTask(prj, t.Fields)
	case "vg-convert":
		err = RunVGConvertTask(prj, t.Fields)

	default:
		log.Printf("unsupported type '%s'", t.Type)
	}
	if err != nil {
		return err
	}

	return nil
}

// AbsExistingPaths gets all the actual filepaths from sources, processes
// wildcards, (including doublestar) and expands all paths relative to basedir
// returns paths only for existing filesystem entries.
func (prj *Project) AbsExistingPaths(sources []string) ([]string, error) {
	var err error
	ret := []string{}
	set := map[string]struct{}{}
	for _, s := range sources {
		if s == "" {
			continue
		}
		s, err = ExpandVariables(s, prj.Vars)
		if err != nil {
			return nil, err
		}

		if !filepath.IsAbs(s) {
			s, err = filepath.Abs(filepath.Join(prj.BaseDir, s))
			if err != nil {
				return nil, err
			}
			s = filepath.Clean(s)
		}

		matches, err := doublestar.FilepathGlob(s)
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			if _, exists := set[m]; exists {
				continue
			}
			ret = append(ret, filepath.ToSlash(m))
			set[m] = struct{}{}
		}
	}
	sort.Strings(ret)
	return ret, nil
}

// AbsPath converts path to absolute path
// non-absolute path is expanded relative to basedir.
func (prj *Project) AbsPath(path string) (string, error) {
	path, err := ExpandVariables(path, prj.Vars)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(path) {
		return filepath.ToSlash(path), nil
	}
	p, err := filepath.Abs(filepath.Join(prj.BaseDir, path))
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Clean(p)), nil
}

func ValidateIdent(s string) (string, error) {
	if s == "" {
		return "", errors.New("invalid identifier: empty")
	}
	for _, c := range s {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z' || c == '_' || c == '-') {
			continue
		} else {
			return "", errors.New("invalid identifier: must only contain A-Z, a-z, _, or -")
		}
	}
	return s, nil
}

type Target struct {
	File    string
	Entry   string
	Content string
}

func (prj *Project) getTarget(m map[string]any) (*Target, error) {
	t := &Target{}
	var err error
	for k, v := range m {
		switch k {
		case "file":
			if s, ok := v.(string); !ok || s == "" {
				return nil, fmt.Errorf("%s: must be a non-empty string", k)
			} else {
				t.File, err = prj.AbsPath(s)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", k, err)
				}
			}
		case "entry":
			if s, ok := v.(string); ok && s != "" {
				t.Entry = s
			} else {
				return nil, fmt.Errorf("%s: must be a non-empty string", k)
			}

		case "content":
			if s, ok := v.(string); ok && s != "" {
				t.Content = s
			} else {
				return nil, fmt.Errorf("%s: must be a non-empty string", k)
			}
		}
	}
	if t.File == "" {
		return nil, fmt.Errorf("missing field: file")
	}
	if t.Entry == "" {
		return nil, fmt.Errorf("missing field: entry")
	}
	if t.Content == "" {
		return nil, fmt.Errorf("missing field: content")
	}

	return t, nil
}

func (prj *Project) GetTargets(n any) ([]*Target, error) {
	tt := []*Target{}
	if m, ok := n.(map[string]any); ok {
		t, err := prj.getTarget(m)
		if err != nil {
			return nil, err
		}
		tt = append(tt, t)
	} else if items, ok := n.([]any); ok {
		for i, item := range items {
			if m, ok := item.(map[string]any); ok {
				t, err := prj.getTarget(m)
				if err != nil {
					return nil, fmt.Errorf("[%d]: %w", i, err)
				}
				tt = append(tt, t)
			}
		}
	} else {
		return nil, errors.New("must be a map or an array of maps")
	}
	return tt, nil
}

func (prj *Project) GetStrings(v any) ([]string, error) {
	ret := []string{}
	if s, ok := v.(string); ok {
		ret = append(ret, s)

	} else if ss, ok := v.([]any); ok {
		for _, elt := range ss {
			if s, ok := elt.(string); ok {
				ret = append(ret, s)
			} else {
				return nil, errors.New("must be a string or an array of strings")
			}
		}
	} else {
		return nil, errors.New("must be a string or an array of strings")
	}
	return ret, nil
}
