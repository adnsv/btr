package tasks

import (
	"fmt"
	"regexp"
	"unicode"
	"unicode/utf8"
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

func naturalCompare(a, b string) int {
	aRunes, bRunes := []rune(a), []rune(b)
	aLen, bLen := len(aRunes), len(bRunes)
	aIndex, bIndex := 0, 0

	for aIndex < aLen && bIndex < bLen {
		aRune, aWidth := utf8.DecodeRuneInString(a[aIndex:])
		bRune, bWidth := utf8.DecodeRuneInString(b[bIndex:])
		if aWidth != bWidth {
			return aWidth - bWidth
		}
		if aRune != bRune {
			// Handle digit characters.
			if unicode.IsDigit(aRune) && unicode.IsDigit(bRune) {
				aNum, bNum := aRune-'0', bRune-'0'
				for aIndex < aLen {
					aRune, aWidth = utf8.DecodeRuneInString(a[aIndex:])
					if !unicode.IsDigit(aRune) {
						break
					}
					aNum = aNum*10 + (aRune - '0')
					aIndex += aWidth
				}
				for bIndex < bLen {
					bRune, bWidth = utf8.DecodeRuneInString(b[bIndex:])
					if !unicode.IsDigit(bRune) {
						break
					}
					bNum = bNum*10 + (bRune - '0')
					bIndex += bWidth
				}
				if aNum != bNum {
					return int(aNum - bNum)
				}
			} else {
				return int(aRune - bRune)
			}
		}
		aIndex += aWidth
		bIndex += bWidth
	}

	// If we reached the end of one string but not the other, compare the lengths.
	if aIndex < aLen {
		return 1
	} else if bIndex < bLen {
		return -1
	} else {
		return 0
	}
}