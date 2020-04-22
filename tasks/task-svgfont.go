package tasks

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/adnsv/iconfont-utils/desc"
	"github.com/adnsv/svg"
)

type FontConfig struct {
	FirstCodePoint string `json:"firstCodePoint"`
	Height         *int   `json:"height"`
	Descent        *int   `json:"descent"`
	Family         string `json:"family"`
}

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

func RunSVGFont(task *Task, basedir string, verbose bool) error {
	if task.Font == nil {
		return errors.New("missing font section\nadd \"font\": { ... } to your task descriptor")
	}
	s := task.Font.FirstCodePoint
	if len(s) == 0 {
		return errors.New("missing firstCodePoint parameter\nspecify \"firstCodePoint\": \"value\" (value is a decimal or hex number) in the task description")
	}
	cp, err := ParseCodepoint(task.Font.FirstCodePoint)

	dst := task.Target
	if len(dst) == 0 {
		return errors.New("missing target path\nplease specify \"target\": \"filepath\" in the task description")
	}
	if !filepath.IsAbs(dst) {
		dst = filepath.Join(basedir, dst)
		dst, err = filepath.Abs(dst)
		if err != nil {
			return err
		}
	}

	sources := task.GetSources()
	if len(sources) == 0 {
		return errors.New("missing icon SVG sources\nspecify \"source\": \"path\": \"path\" or \"sources\": [\"path\",...] in the task description")
	}
	filepaths, err := ObtainFilePaths(basedir, sources)
	if err != nil {
		return err
	}
	glyphs := []*Glyph{}
	for _, fn := range filepaths {
		if verbose {
			fmt.Printf("loading %q\n", fn)
		}
		g, err := readGlyph(fn)
		if err != nil {
			return err
		}
		glyphs = append(glyphs, g)
	}

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
	name := task.Font.Family
	if name == "" {
		name = filepath.Base(dst)
		name = name[:len(name)-len(filepath.Ext(name))]
	}
	svgf, err := makeSVGFont(glyphs, ascent, descent, name)
	if err != nil {
		return err
	}
	if verbose {
		fmt.Printf("writing %q\n", dst)
	}
	err = ioutil.WriteFile(dst, []byte(svgf), 0644)
	if err != nil {
		return err
	}

	return nil
}

type Glyph struct {
	FileName  string
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
	sg, err := svg.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	g := &Glyph{}
	if sg.Width == nil {
		return nil, fmt.Errorf("Missing width attr")
	}
	if sg.Height == nil {
		return nil, fmt.Errorf("Missing height attr")
	}
	if len(sg.Items) == 0 {
		return nil, fmt.Errorf("Missing elements")
	}
	path, ok := sg.Items[0].(*svg.Path)
	if !ok || path == nil {
		return nil, fmt.Errorf("Does not have path element")
	}

	g.FileName = fn
	g.Width = sg.Width.Value
	g.Height = sg.Height.Value
	g.Path = path.D
	g.Transform = path.Transform

	_, err = svg.ParsePath(g.Path)
	if err != nil {
		return nil, err
	}

	return g, nil
}

func makeSVGFont(glyphs []*Glyph, fontAscent, fontDescent int, fontName string) (string, error) {
	fontHeight := fontAscent + fontDescent
	horizAdvX := fontHeight

	s := fmt.Sprintf(`<svg xmlns='http://www.w3.org/2000/svg'>
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
			return "", err
		}

		scale := float64(fontHeight) / g.Height
		for i := range pd.Vertices {
			pd.Vertices[i].X = math.Round(pd.Vertices[i].X * scale)
			pd.Vertices[i].Y = math.Round((g.Height-pd.Vertices[i].Y)*scale - float64(fontDescent))
		}
		adv := int(math.Round(g.Width * scale))

		name := filepath.Base(g.FileName)
		ext := filepath.Ext(name)
		name = name[:len(name)-len(ext)]

		pds := pd.String()
		s += fmt.Sprintf(`
	<glyph
		glyph-name="%s"
		unicode="&#x%x;"
		d="%s"
		horiz-adv-x="%d" />`,
			name, g.CodePoint, pds, adv)
	}

	s += `
	</font>
	</defs>
	</svg>`

	return s, nil
}

type SVGFont struct {
	XMLName xml.Name    `xml:"svg"`
	Glyphs  []*SVGGlyph `xml:"defs>font>glyph"`
}

type SVGGlyph struct {
	Name    string `xml:"glyph-name,attr"`
	Unicode string `xml:"unicode,attr"`
}

func makeHeader(src, dst, ns string) error {
	buf, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	font := &SVGFont{}
	err = xml.Unmarshal(buf, &font)
	if err != nil {
		return err
	}
	icons := []*desc.Icon{}
	for _, g := range font.Glyphs {
		runes := []rune(g.Unicode)
		if len(runes) == 1 {
			icons = append(icons, &desc.Icon{
				Name:      g.Name,
				ID:        g.Name,
				Codepoint: runes[0]})
		}
	}
	gen := desc.Generator{TypePrefix: "constexpr const char*", Namespace: ns}
	buf = gen.Produce(icons)
	err = ioutil.WriteFile(dst, buf, 644)
	if err != nil {
		return err
	}
	return nil
}
