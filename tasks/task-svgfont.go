package tasks

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/adnsv/svg"
	"golang.org/x/exp/maps"
)

// ParseCodepoint extracts unicode codepoint value from a string which can
// be a decimal number or a hex (U+0000 or 0x0000 or 0X0000)
func ParseCodepoint(s string) (rune, error) {
	var cp uint64
	var err error
	if strings.HasPrefix(s, "U+") {
		cp, err = strconv.ParseUint(s[2:], 16, 32)
	} else if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		cp, err = strconv.ParseUint(s[2:], 16, 32)
	} else {
		cp, err = strconv.ParseUint(s, 10, 32)
	}
	if err != nil {
		return 0, fmt.Errorf("invalid codepoint value %q, %w", s, err)
	}
	return rune(cp), nil
}

func RunSVGFontMake(task *Task, config *Config) error {
	if task.Font == nil {
		return errors.New("missing font section\nadd \"font\": { ... } to your task descriptor")
	}
	s := task.Font.FirstCodePoint
	if len(s) == 0 {
		return errors.New("missing firstCodePoint parameter\nspecify \"firstCodePoint\": \"value\" (value is a decimal or hex number) in the task description")
	}
	cp, err := ParseCodepoint(task.Font.FirstCodePoint)
	if err != nil {
		return err
	}

	dst := task.Target
	if len(dst) == 0 {
		return errors.New("missing target path\nplease specify \"target\": \"filepath\" in the task description")
	}
	if !filepath.IsAbs(dst) {
		dst = filepath.Join(config.BaseDir, dst)
		dst, err = filepath.Abs(dst)
		if err != nil {
			return err
		}
	}

	sources := task.GetSources()
	if len(sources) == 0 {
		return errors.New("missing sources paths\nspecify \"source\": \"path\" or \"sources\": [\"path\",...] in the task description")
	}
	sourcepaths, err := AbsExistingPaths(config.BaseDir, sources)
	if err != nil {
		return err
	}
	glyphs := []*Glyph{}

	maxPathLength := 0
	for _, fn := range sourcepaths {
		if len(fn) > maxPathLength {
			maxPathLength = len(fn)
		}
	}

	for _, fn := range sourcepaths {
		gname := RemoveExtension(filepath.Base(fn))
		gname = MakeIdentStr(gname)

		if config.Verbose {
			n := maxPathLength - len(fn) + 1
			if n < 1 {
				n = 1
			}
			fmt.Printf("loading %q%s-> %s\n", fn, strings.Repeat(" ", n), gname)
		}
		g, err := readGlyph(fn)
		if err != nil {
			return err
		}
		g.Name = gname
		glyphs = append(glyphs, g)
	}

	sort.SliceStable(glyphs, func(i, j int) bool {
		return strings.Compare(glyphs[i].Name, glyphs[j].Name) < 0
	})

	// assign codepoints
	for _, g := range glyphs {
		g.CodePoint = rune(cp)
		cp++
	}

	height := 512
	if task.Font.Height != nil {
		height = *task.Font.Height
	}
	descent := 128
	if task.Font.Descent != nil {
		descent = *task.Font.Descent
	}

	ascent := height - descent
	familyname := task.Font.Family
	if familyname == "" {
		familyname = filepath.Base(dst)
		familyname = familyname[:len(familyname)-len(filepath.Ext(familyname))]
	}
	out := bytes.Buffer{}
	err = makeSVGFont(&out, glyphs, ascent, descent, familyname)
	if err != nil {
		return err
	}

	fmt.Printf("writing %s ... ", dst)
	err = os.WriteFile(dst, out.Bytes(), 0x666)
	if err == nil {
		fmt.Printf("SUCCEEDED\n")
	} else {
		fmt.Printf("FAILED\n")
	}

	return err
}

type Glyph struct {
	FilePath  string
	Name      string
	CodePoint rune
	Width     float64
	Height    float64
	Path      string
	Transform *svg.Transform
}

func readGlyph(fn string) (*Glyph, error) {
	data, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	sg, err := svg.Parse(string(data))
	if err != nil {
		return nil, err
	}
	g := &Glyph{}

	vb, err := sg.ViewBox.Parse()
	if err != nil {
		return nil, fmt.Errorf("bad svg.viewBox attribute: %s", err)
	}
	var u svg.Units
	w := vb.Width
	h := vb.Height

	if sg.Width != "" {
		w, u, err = sg.Width.AsNumeric()
		if err != nil {
			return nil, fmt.Errorf("bad svg.width attribute: %w", err)
		} else if u != svg.UnitNone && u != svg.UnitPX {
			return nil, fmt.Errorf("bad svg.width attribute: unexpected units")
		}
	}
	if sg.Height != "" {
		h, u, err = sg.Height.AsNumeric()
		if err != nil {
			return nil, fmt.Errorf("bad svg.height attribute: %w", err)
		} else if u != svg.UnitNone && u != svg.UnitPX {
			return nil, fmt.Errorf("bad svg.height attribute: unexpected units")
		}
	}

	if len(sg.Items) == 0 {
		g.FilePath = fn
		g.Width = w
		g.Height = h
		g.Path = ""
		g.Transform = nil
		return g, nil
	}
	path, ok := sg.Items[0].(*svg.Path)
	if !ok || path == nil {
		return nil, fmt.Errorf("does not have path element")
	}

	g.FilePath = fn
	g.Width = w
	g.Height = h
	g.Path = path.D
	g.Transform = path.Transform

	_, err = svg.ParsePath(g.Path)
	if err != nil {
		return nil, err
	}

	return g, nil
}

func makeSVGFont(out io.Writer, glyphs []*Glyph,
	fontAscent, fontDescent int, fontName string) error {

	fontHeight := fontAscent + fontDescent
	horizAdvX := fontHeight

	fmt.Fprintf(out, `<svg xmlns='http://www.w3.org/2000/svg'>
<defs>
<font id="%s" horiz-adv-x="%d">
	<font-face
		font-family="%s"
		font-weight="400"
		font-stretch="normal"
		units-per-em="%d"
		ascent="%d"
		descent="%d" />
	<missing-glyph horiz-adv-x="%d" />`,
		fontName, horizAdvX,
		fontName,
		fontHeight, fontAscent, fontDescent, horizAdvX)

	for _, g := range glyphs {
		pd, err := svg.ParsePath(g.Path)
		if err != nil {
			return err
		}

		scale := float64(fontHeight) / g.Height
		for i := range pd.Vertices {
			pd.Vertices[i].X = math.Round(pd.Vertices[i].X * scale)
			pd.Vertices[i].Y = math.Round((g.Height-pd.Vertices[i].Y)*scale - float64(fontDescent))
		}
		adv := int(math.Round(g.Width * scale))

		pds := pd.String()
		fmt.Fprintf(out, `
	<glyph
		glyph-name="%s"
		unicode="&#x%x;"
		d="%s"
		horiz-adv-x="%d" />`,
			g.Name, g.CodePoint, pds, adv)
	}

	fmt.Fprintf(out, "\n</font>\n</defs>\n</svg>/n")
	return nil
}

func RunSVGFontHPP(task *Task, config *Config) error {
	src := task.Source
	if src == "" {
		return errors.New("missing 'source' or 'sources' field")
	}
	dst := task.Target
	if dst == "" {
		return errors.New("missing 'target' or 'targets' field")
	}
	if task.HppTemplate == "" {
		return errors.New("missing 'hpp-template' field")
	}
	if task.HppEntryTemplate == "" {
		return errors.New("missing 'entry-template' field")
	}

	var err error
	if !filepath.IsAbs(src) {
		src = filepath.Join(config.BaseDir, src)
		src, err = filepath.Abs(src)
		if err != nil {
			return err
		}
	}

	if !filepath.IsAbs(dst) {
		dst = filepath.Join(config.BaseDir, dst)
		dst, err = filepath.Abs(dst)
		if err != nil {
			return err
		}
	}

	if config.Verbose {
		fmt.Printf("reading %q\n", src)
	}

	buf := bytes.Buffer{}
	out := tabwriter.NewWriter(&buf, 0, 4, 1, ' ', 0)

	vv := maps.Clone(config.Vars)
	err = generateSVGFontHPP(out, src, vv, task.HppTemplate, task.EntryTemplate)
	if err != nil {
		return err
	}
	out.Flush()

	fmt.Printf("writing %s ... ", dst)
	err = os.WriteFile(dst, buf.Bytes(), 0x666)
	if err == nil {
		fmt.Printf("SUCCEEDED\n")
	} else {
		fmt.Printf("FAILED\n")
	}

	return err
}

type SVGFont struct {
	XMLName xml.Name    `xml:"svg"`
	Glyphs  []*SVGGlyph `xml:"defs>font>glyph"`
}

type SVGGlyph struct {
	Name    string `xml:"glyph-name,attr"`
	Unicode string `xml:"unicode,attr"`
}

func generateSVGFontHPP(out io.Writer, svgfn string, globalVars map[string]string, hppTemplate, entryTemplate string) error {
	buf, err := ioutil.ReadFile(svgfn)
	if err != nil {
		return err
	}
	font := &SVGFont{}
	err = xml.Unmarshal(buf, &font)
	if err != nil {
		return err
	}

	first := true
	cpmin := rune(0)
	cpmax := cpmin
	for _, g := range font.Glyphs {
		runes := []rune(g.Unicode)
		if len(runes) != 1 {
			continue
		}
		cp := runes[0]
		if first {
			first = false
			cpmin = cp
			cpmax = cp
		} else {
			if cp > cpmax {
				cpmax = cp
			}
			if cp < cpmin {
				cpmin = cp
			}
		}
	}
	if first {
		return errors.New("no codepoints found")
	}

	entryLines := []string{}
	for _, g := range font.Glyphs {
		runes := []rune(g.Unicode)

		if len(runes) != 1 {
			continue
		}
		cp := runes[0]
		u8 := string([]rune{cp})
		escaped := ""
		for i := 0; i < len(u8); i++ {
			escaped += fmt.Sprintf(`\x%x`, u8[i])
		}
		hex := fmt.Sprintf("%X", cp)

		entryVars := map[string]string{
			"name":    g.Name,
			"ident":   MakeIdentStr(g.Name),
			"unicode": hex,
			"escaped": escaped,
		}

		line, err := ExpandVariables(entryTemplate, entryVars)
		if err != nil {
			return fmt.Errorf("entry template: %w", err)
		}

		entryLines = append(entryLines, line)
	}

	globalVars["codepoint-min"] = fmt.Sprintf("%X", cpmin)
	globalVars["codepoint-max"] = fmt.Sprintf("%X", cpmax)
	globalVars["entries"] = strings.Join(entryLines, "\n")

	fileContent, err := ExpandVariables(hppTemplate, globalVars)
	if err != nil {
		return fmt.Errorf("hpp-template: %w", err)
	}

	_, err = out.Write([]byte(fileContent))
	return err
}

func RunSVGFontTTF(task *Task, config *Config) error {
	src := task.Source
	if src == "" {
		return errors.New("missing 'source' or 'sources' field")
	}
	dst := task.Target
	if len(dst) == 0 {
		return errors.New("missing 'target' or 'targets' field")
	}

	var err error
	if !filepath.IsAbs(src) {
		src = filepath.Join(config.BaseDir, src)
		src, err = filepath.Abs(src)
		if err != nil {
			return err
		}
	}

	if !filepath.IsAbs(dst) {
		dst = filepath.Join(config.BaseDir, dst)
		dst, err = filepath.Abs(dst)
		if err != nil {
			return err
		}
	}

	if config.Verbose {
		fmt.Printf("source %q\n", src)
		fmt.Printf("target %q\n", src)
	}

	cmd := exec.Command("svg2ttf", "--version")
	_, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed to execute svg2ttf utility\n")
		log.Printf("Please make sure it is installed:\n")
		log.Printf("npm install -g svg2ttf\n")
		log.Printf("you will need to have node.js installed\n")
		return err
	}
	cmd = exec.Command("svg2ttf", src, dst)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("svg2ttf: %w", err)
	}

	return nil
}

func RemoveExtension(fn string) string {
	ext := filepath.Ext(fn)
	return fn[:len(fn)-len(ext)]
}

func MakeIdentStr(s string) string {
	s = strings.ReplaceAll(s, "-", "_")
	if s == "" {
		return "_"
	}
	if !identStart(s[0]) {
		s = "_" + s
	}
	for _, kw := range reservedKeywords {
		if s == kw {
			s += "_"
			break
		}
	}
	return s
}

func identStart(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
}

var reservedKeywords = [...]string{
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
	"_Thread_local"}
