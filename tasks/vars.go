package tasks

import (
	"fmt"
	"regexp"
)

var dollar_curly_re = regexp.MustCompile(`\$\{([_a-zA-Z][-_a-zA-Z0-9]*)\}`)

func ExpandVariables(s string, vars map[string]string) (string, error) {
	var err error
	return dollar_curly_re.ReplaceAllStringFunc(s, func(m string) string {
		if len(m) >= 3 && m[0] == '$' && m[1] == '{' && m[len(m)-1] == '}' {
			m = m[2 : len(m)-1]
		}
		if val, ok := vars[m]; ok {
			return val
		} else {
			err = fmt.Errorf("unknown variable %s", m)
			return ""
		}
	}), err
}
