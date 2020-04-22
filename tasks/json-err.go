package tasks

import (
	"encoding/json"
	"fmt"
)

// lineAndCharacter locates line and pos from offset into a file
func lineAndCharacter(input string, offset int) (line int, character int, err error) {
	lf := rune(0x0A)
	if offset > len(input) || offset < 0 {
		return 0, 0, fmt.Errorf("can't find offset %d within the input", offset)
	}

	for i, b := range input {
		if b == lf {
			line++
			character = 0
		}
		character++
		if i == offset {
			break
		}
	}
	return line, character, nil
}

// jsonErrDetail ammends an error returned from json.Unmarshal with
// line:position info.
func jsonErrDetail(input string, err error) error {
	switch v := err.(type) {
	case *json.UnmarshalTypeError:
		line, pos, lcErr := lineAndCharacter(input, int(v.Offset))
		if lcErr != nil {
			return err
		}
		return fmt.Errorf("at line %d:%d %s", line+1, pos, err)
	case *json.SyntaxError:
		line, pos, lcErr := lineAndCharacter(input, int(v.Offset))
		if lcErr != nil {
			return err
		}
		return fmt.Errorf("at line %d:%d %s", line+1, pos, err)
	default:
		return err
	}
}
