package tasks

import (
	"fmt"
	"os"
)

func RunFileTask(prj *Project, fields map[string]any) error {
	target_fn := ""
	content := ""
	var err error
	for k, v := range fields {
		switch k {
		case "target":
			if s, ok := v.(string); ok && s != "" {
				target_fn, err = prj.AbsPath(s)
				if err != nil {
					return fmt.Errorf("%s: %w", k, err)
				}
			} else {
				return fmt.Errorf("%s: must be a non-empty string", k)
			}
		case "content":
			if s, ok := v.(string); ok {
				target_fn, err = prj.AbsPath(s)
				if err != nil {
					return fmt.Errorf("%s: %w", k, err)
				}
			} else {
				return fmt.Errorf("%s: must be a string", k)
			}
		default:
			fmt.Printf("warning: unknown field '%s'\n", k)
		}
	}

	content, err = ExpandVariables(content, prj.Vars)
	if err != nil {
		return err
	}

	fmt.Printf("writing %s ... ", target_fn)
	err = os.WriteFile(target_fn, []byte(content), 0666)
	if err == nil {
		fmt.Printf("SUCCEEDED\n")
	} else {
		fmt.Printf("FAILED\n")
	}
	return nil
}
