package tasks

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func RunDirTask(prj *Project, fields map[string]any) error {
	var path string
	var varname string
	if_missing := "create"
	if_exists := ""

	var err error
	for k, v := range fields {
		switch k {
		case "path":
			if s, ok := v.(string); ok && s != "" {
				path, err = prj.AbsPath(s)
				if err != nil {
					return fmt.Errorf("path: %w", err)
				}
			} else {
				return fmt.Errorf("path: must be a non-empty string")
			}

		case "if-missing":
			if s, ok := v.(string); !ok {
				return fmt.Errorf("if-missing: must be one of 'create', 'error'")
			} else if s == "create" || s == "error" {
				if_missing = s
			} else {
				return fmt.Errorf("if-missing: must be one of 'create', 'error'")
			}

		case "if-exists":
			if s, ok := v.(string); !ok {
				return fmt.Errorf("if-exists: must be one of 'clean', 'error'")
			} else if s == "clean" || s == "error" {
				if_exists = s
			} else {
				return fmt.Errorf("if-exists: must be one of 'clean', 'error'")
			}

		case "var":
			if s, ok := v.(string); ok && s != "" {
				varname = s
			} else {
				return fmt.Errorf("var must be a non-empty identifier")
			}

		default:
			fmt.Printf("warning: unknown field '%s'\n", k)
		}
	}

	if path == "" {
		path = filepath.Join(os.TempDir(), "btr")
		if prj.Verbose {
			fmt.Printf("using temporary dir '%s'", path)
		}
	}

	if varname != "" {
		if _, exists := prj.Vars[varname]; exists {
			return fmt.Errorf("variable '%s' already exists", varname)
		}
		prj.Vars[varname] = path
	}

	stat, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		if if_missing == "error" {
			return fmt.Errorf("directory '%s' does not exist", path)
		} else {
			// assume if_missing = create
			if prj.Verbose {
				fmt.Printf("creating directory '%s'", path)
			}
			err := os.MkdirAll(path, 0755)
			if err != nil {
				return fmt.Errorf("failed to create directory '%s': %w", path, err)
			}
		}
	} else if err != nil {
		return fmt.Errorf("path '%s': %w", path, err)
	} else if !stat.IsDir() {
		return fmt.Errorf("path '%s' is not a directory", path)
	} else {
		// directory already exists
		if if_exists == "error" {
			return fmt.Errorf("directory '%s' exists", path)
		} else if if_exists == "clean" {
			rel, err := filepath.Rel(filepath.Dir(prj.BaseDir), path)
			if err != nil {
				return fmt.Errorf("path: %w", err)
			}
			if rel == "" || rel[0] == '.' {
				// a bit of safety: don't delete self and don't delete external paths
				return fmt.Errorf("external path '%s' is not allowed when if_exists=clean", path)
			}

			entries, err := os.ReadDir(path)
			if err != nil {
				return fmt.Errorf("path: %w", err)
			}
			if len(entries) > 0 {
				if prj.Verbose {
					fmt.Printf("removing existing content in '%s'\n", path)
				}
				for _, entry := range entries {
					subpath := filepath.Join(path, entry.Name())
					err = os.RemoveAll(subpath)
					if err != nil {
						return fmt.Errorf("path: %w", err)
					}
				}
			}
		}
	}

	return nil
}
