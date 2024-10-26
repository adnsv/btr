package tasks

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"
	"unicode"
	"unicode/utf8"
)

func RemoveExtension(fn string) string {
	ext := filepath.Ext(fn)
	return fn[:len(fn)-len(ext)]
}

// ReplaceExtension replaces the extension of a filename with a new one.
func ReplaceExtension(filename, newExt string) string {
	if filename == "" {
		return ""
	}

	// Get the base name without extension
	base := filename
	if ext := filepath.Ext(filename); ext != "" {
		base = strings.TrimSuffix(filename, ext)
	}

	return base + newExt
}

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

func identStart(c rune) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
}

func identChar(c rune) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}

func MakeCPPIdentStr(s string) string {
	if s == "" {
		return "_"
	}

	ret := ""
	for i, r := range s {
		if i == 0 {
			if identStart(r) {
				ret += string(r)
			} else {
				ret += "_"
			}
		} else {
			if identChar(r) {
				ret += string(r)
			} else {
				ret += "_"
			}
		}
	}
	for _, kw := range cppReservedKeywords {
		if s == kw {
			s += "_"
			break
		}
	}
	return ret
}

var cppReservedKeywords = [...]string{
	"auto",
	"break",
	"case",
	"char",
	"const",
	"continue",
	"default",
	"do",
	"double",
	"else",
	"enum",
	"extern",
	"float",
	"for",
	"goto",
	"if",
	"inline",
	"int",
	"long",
	"register",
	"restrict",
	"return",
	"short",
	"signed",
	"sizeof",
	"static",
	"struct",
	"switch",
	"typedef",
	"union",
	"unsigned",
	"void",
	"volatile",
	"while",
	"_Alignas ",
	"_Alignof",
	"_Atomic",
	"_Bool",
	"_Complex ",
	"_Generic",
	"_Imaginary",
	"_Noreturn",
	"_Static_assert",
	"_Thread_local",
	"import",
	"export"}

func CommaWrap(segments []string, indent string, limit int) string {
	ret := ""

	ln := ""
	for _, s := range segments {
		if len(ln) == 0 {
			ln = s + ","
		} else if len(ln)+2+len(s) <= limit {
			ln += " " + s + ","
		} else {
			if len(ret) > 0 {
				ret += "\n"
			}
			ret += indent + ln
			ln = s + ","
		}
	}
	if len(ln) > 0 {
		if len(ret) > 0 {
			ret += "\n"
		}
		ret += indent + ln
	}
	return ret
}

func StringQWrap(s string, indent string, limit int) string {
	if limit > 2 {
		limit -= 2
	}
	ret := ""
	for len(s) > limit {
		if len(ret) > 0 {
			ret += "\n"
		}
		ret += indent + fmt.Sprintf("%q", s[:limit])
		s = s[limit:]
	}
	if len(s) > 0 {
		if len(ret) > 0 {
			ret += "\n"
		}
		ret += indent + fmt.Sprintf("%q", s)
	}
	return ret
}

func bytesToHexWrappedIndented(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// Pre-calculate the final string length to avoid reallocations
	// For each byte we need: 2 chars for hex + 1 char for comma = 3 chars
	// Plus we need a newline every 32 bytes
	// Subtract 1 for the last comma we won't need
	capacity := len(data)*3 + (len(data)-1)/32 - 1

	var builder strings.Builder
	builder.Grow(capacity)
	builder.WriteString("    ")

	// Lookup table for hex conversion - much faster than fmt.Sprintf
	const hexChars = "0123456789abcdef"

	for i, b := range data {
		if i > 0 {
			builder.WriteByte(',')
			// Add newline after every 32 bytes
			if i%32 == 0 {
				builder.WriteString("\n    ")
			}
		}
		builder.WriteByte('0')
		builder.WriteByte('x')
		builder.WriteByte(hexChars[b>>4])
		builder.WriteByte(hexChars[b&0x0F])
	}

	return builder.String()
}

// HppCppNs contains options for codegen of a c++ compilation unit.
type HppCppNs struct {
	HppTarget string // hpp filename
	CppTarget string // cpp filename
	Namespace string // namespace
}

func FetchCppTargetFields(prj *Project, fields map[string]any) (HppCppNs, error) {
	ret := HppCppNs{}

	var err error
	for k, v := range fields {
		switch k {

		case "hpp-target":
			ret.HppTarget, err = prj.GetString(v, true)
			if err != nil {
				return ret, fmt.Errorf("%s: %w", k, err)
			}
			if ret.HppTarget == "" {
				return ret, fmt.Errorf("%s: empty value is not allowed", k)
			}

		case "cpp-target":
			ret.CppTarget, err = prj.GetString(v, true)
			if err != nil {
				return ret, fmt.Errorf("%s: %w", k, err)
			}
			if ret.CppTarget == "" {
				return ret, fmt.Errorf("%s: empty value is not allowed", k)
			}

		case "namespace":
			ret.Namespace, err = prj.GetString(v, true)
			if err != nil {
				return ret, fmt.Errorf("%s: %w", k, err)
			}
		}
	}

	if ret.HppTarget == "" && ret.CppTarget == "" {
		return ret, fmt.Errorf("missing fields: hpp-target and/or cpp-target")
	}

	if ret.HppTarget == "" {
		// generate hpp name from cpp
		ret.HppTarget = ReplaceExtension(ret.CppTarget, ".hpp")
		if ret.CppTarget == ret.HppTarget {
			return ret, fmt.Errorf("can't auto-generate hpp-target from cpp-target")
		}
	} else if ret.CppTarget == "" {
		ret.CppTarget = ReplaceExtension(ret.HppTarget, ".cpp")
		if ret.CppTarget == ret.HppTarget {
			return ret, fmt.Errorf("can't auto-generate cpp-target from hpp-target")
		}
	} else if ret.CppTarget == ret.HppTarget {
		return ret, fmt.Errorf("hpp-target must be different from cpp-target")
	}

	ret.HppTarget, err = prj.AbsPath(ret.HppTarget)
	if err != nil {
		return ret, err
	}
	ret.CppTarget, err = prj.AbsPath(ret.CppTarget)
	if err != nil {
		return ret, err
	}

	return ret, nil
}

type SourceFileWriter struct {
	path      string
	namespace string
	t         *tabwriter.Writer
	b         bytes.Buffer
}

func (w *SourceFileWriter) Write(p []byte) (n int, err error) {
	return w.t.Write(p)
}

func (v *HppCppNs) MakeWriters() (hpp *SourceFileWriter, cpp *SourceFileWriter) {
	hpp = &SourceFileWriter{}
	hpp.path = v.HppTarget
	hpp.namespace = v.Namespace
	hpp.t = tabwriter.NewWriter(&hpp.b, 0, 4, 4, ' ', 0)
	cpp = &SourceFileWriter{}
	cpp.path = v.CppTarget
	cpp.namespace = v.Namespace
	cpp.t = tabwriter.NewWriter(&cpp.b, 0, 4, 4, ' ', 0)
	return
}

func (v *HppCppNs) PutFileHeader(hpp *SourceFileWriter, cpp *SourceFileWriter) {
	if hpp != nil {
		fmt.Fprintf(hpp, "#pragma once\n\n")
		fmt.Fprintf(hpp, "// DO NOT EDIT: Generated file\n")
		fmt.Fprintf(hpp, "// clang-format off\n\n")
	}

	if cpp != nil {
		fmt.Fprintf(cpp, "// DO NOT EDIT: Generated file\n")
		fmt.Fprintf(cpp, "// clang-format off\n\n")
	}
}

func (v *SourceFileWriter) StartNamespace() {
	if v.namespace != "" {
		fmt.Fprintf(v.t, "namespace %s {\n\n", v.namespace)
	}
}

func (v *SourceFileWriter) DoneNamespace() {
	if v.namespace != "" {
		fmt.Fprintf(v.t, "} // namespace %s\n", v.namespace)
	}
}

func (v *SourceFileWriter) WriteOutFile() error {
	v.t.Flush()
	fmt.Printf("- writing %s ... ", v.path)
	err := os.WriteFile(v.path, v.b.Bytes(), 0666)
	if err == nil {
		fmt.Printf("SUCCEEDED\n")
	} else {
		fmt.Printf("FAILED\n")
		return fmt.Errorf("when writing %s: %w", v.path, err)
	}
	return nil
}

func (v *SourceFileWriter) RelPathTo(other *SourceFileWriter) string {
	if v == nil || other == nil {
		return "#ERR"
	}
	rel, err := filepath.Rel(filepath.Dir(v.path), other.path)
	if err != nil {
		return "#ERR"
	}
	return rel
}
