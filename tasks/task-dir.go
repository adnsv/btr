package tasks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func RunDir(task *Task, config *Config, cmd string) error {
	targets := task.GetTargets()
	if len(targets) == 0 {
		return errors.New("missing target paths\nspecify \"target\": \"path\" or \"targets\": [\"path\",...] in the task description")
	}

	dirs, err := AbsPaths(config.BaseDir, targets)
	if err != nil {
		return err
	}

	switch cmd {
	case "clean":
		for _, dir := range dirs {
			rel, err := filepath.Rel(filepath.Dir(config.BaseDir), dir)
			if err != nil {
				return err
			}
			if rel == "" || rel[0] == '.' {
				// a bit of safety: don't delete self and don't delete external paths
				return fmt.Errorf("failed 'dir.clean': external path '%s' is not allowed", dir)
			}
			err = os.RemoveAll(dir)
			if err == nil {
				err = os.MkdirAll(dir, 0744)
			}
			if err != nil {
				return err
			}
			if config.Verbose {
				fmt.Printf("cleaned dir: %q\n", dir)
			}
		}

	case "make":
		for _, dir := range dirs {
			err := os.MkdirAll(dir, 0744)
			if err != nil {
				return err
			}
			if config.Verbose {
				fmt.Printf("created dir: %q\n", dir)
			}
		}

	default:
		return fmt.Errorf("invalid dir task command %q", cmd)
	}
	return nil
}
