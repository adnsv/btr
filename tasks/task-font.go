package tasks

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
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

// Convert RunSVGFontTask to struct
type SVGFontTask struct{}

func (SVGFontTask) Run(prj *Project, fields map[string]any) error {
	sources := []string{}
	target_fn := ""
	html_preview_fn := ""
	codepoint := rune(0xf000)
	height := 512
	var optDescent *int
	family := ""

	var err error
	for k, v := range fields {
		switch k {
		case "source":
			sources, err = prj.GetStrings(v)
			if err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}

		case "target":
			if s, ok := v.(string); ok && s != "" {
				target_fn, err = prj.AbsPath(s)
				if err != nil {
					return fmt.Errorf("%s: %w", k, err)
				}
			} else {
				return fmt.Errorf("%s: must be a non-empty string", k)
			}
		case "html-preview":
			if s, ok := v.(string); ok && s != "" {
				html_preview_fn, err = prj.AbsPath(s)
				if err != nil {
					return fmt.Errorf("%s: %w", k, err)
				}
			} else {
				return fmt.Errorf("%s: must be a non-empty string", k)
			}

		case "first-codepoint":
			if s, ok := v.(string); ok {
				codepoint, err = parseCodepoint(s)
				if err != nil {
					return fmt.Errorf("%s: %w", k, err)
				}
			}

		case "height", "font-height":
			if v, ok := v.(int); ok && v > 0 {
				height = v
			} else {
				return fmt.Errorf("%s: must be a positive integer", k)
			}

		case "descent", "font-descent":
			if v, ok := v.(int); ok {
				optDescent = &v
			} else {
				return fmt.Errorf("%s: must be an integer", k)
			}

		case "family", "family-name":
			if s, ok := v.(string); ok && s != "" {
				family = s
			} else {
				return fmt.Errorf("%s: must be a non-empty string", k)
			}

		default:
			fmt.Printf("- WARNING: unknown field '%s'\n", k)
		}
	}

	var descent = 0
	if optDescent != nil {
		descent = *optDescent
	} else {
		descent = height * 20 / 100 // 20% by default
	}

	if prj.Verbose {
		fmt.Printf("- font height:  %d\n", height)
		fmt.Printf("- font descent: %d\n", descent)
	}

	if target_fn == "" {
		return fmt.Errorf("missing field: target")
	}

	if len(sources) == 0 {
		return fmt.Errorf("missing field: source")
	}

	source_fns, err := prj.AbsExistingPaths(sources)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	} else if len(source_fns) == 0 {
		return fmt.Errorf("no sources found")
	}

	glyphs := []*Glyph{}

	maxPathLength := 0
	for _, fn := range source_fns {
		if len(fn) > maxPathLength {
			maxPathLength = len(fn)
		}
	}

	for _, fn := range source_fns {
		gname := RemoveExtension(filepath.Base(fn))
		gname = strings.ReplaceAll(gname, " ", "-")

		if prj.Verbose {
			n := maxPathLength - len(fn) + 1
			if n < 1 {
				n = 1
			}
			fmt.Printf("- reading: %s%s-> %s\n", fn, strings.Repeat(" ", n), gname)
		}
		g, err := readSVGFileAsGlyph(fn)
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
		g.CodePoint = codepoint
		codepoint++
	}

	ascent := height - descent
	if family == "" {
		family = filepath.Base(target_fn)
		family = family[:len(family)-len(filepath.Ext(family))]
	}
	if prj.Verbose {
		fmt.Printf("- family: %s\n", family)
	}
	out := bytes.Buffer{}
	err = composeGlyphsIntoSVGFont(&out, glyphs, ascent, descent, family)
	if err != nil {
		return err
	}

	fmt.Printf("- writing %s ... ", target_fn)
	err = os.WriteFile(target_fn, out.Bytes(), 0666)
	if err == nil {
		fmt.Printf("SUCCEEDED\n")
	} else {
		fmt.Printf("FAILED\n")
	}

	if html_preview_fn != "" {
		out.Reset()
		fmt.Fprintf(&out, "<html><body><table>\n")
		rel_base := filepath.Dir(html_preview_fn)
		for _, g := range glyphs {
			fn, _ := filepath.Rel(rel_base, g.FilePath)
			fn = filepath.ToSlash(fn)
			ident := MakeCPPIdentStr(g.Name)
			fmt.Fprintf(&out, "  <tr><td><code>%s</code></td><td><img width='20pt' src='%s'/></td></tr>\n", ident, fn)
		}
		fmt.Fprintf(&out, "</table></body></html>\n")
		fmt.Printf("- writing %s ... ", html_preview_fn)
		err = os.WriteFile(html_preview_fn, out.Bytes(), 0666)
		if err == nil {
			fmt.Printf("SUCCEEDED\n")
		} else {
			fmt.Printf("FAILED\n")
		}
	}

	return err
}

// Convert RunGlyphNamesTask to struct
type GlyphNamesTask struct{}

func (GlyphNamesTask) Run(prj *Project, fields map[string]any) error {
	source_fn := ""
	targets := []*Target{}

	var err error
	for k, v := range fields {
		switch k {
		case "source":
			if s, ok := v.(string); ok && s != "" {
				source_fn, err = prj.AbsPath(s)
				if err != nil {
					return fmt.Errorf("%s: %w", k, err)
				}
			} else {
				return fmt.Errorf("%s: must be a non-empty string", k)
			}

		case "target":
			targets, err = prj.GetTargets(v)
			if err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
			if len(targets) == 0 {
				return fmt.Errorf("%s: must not be empty", k)
			}

		default:
			fmt.Printf("- WARNING: unknown field '%s'\n", k)
		}
	}

	if source_fn == "" {
		return fmt.Errorf("missing field: source")
	}
	if len(targets) == 0 {
		return fmt.Errorf("missing field: target")
	}

	if prj.Verbose {
		fmt.Printf("- reading: %s\n", source_fn)
	}

	glyphs, err := extractNamedCodepoints(source_fn)
	if err != nil {
		return err
	}

	for _, t := range targets {
		buf := bytes.Buffer{}
		out := tabwriter.NewWriter(&buf, 0, 4, 1, ' ', 0)
		err = codegenGlyphNames(out, glyphs, maps.Clone(prj.Vars), t.Content, t.Entry)
		if err != nil {
			return err
		}
		out.Flush()

		fmt.Printf("- writing %s ... ", t.File)
		err = os.WriteFile(t.File, buf.Bytes(), 0666)
		if err == nil {
			fmt.Printf("SUCCEEDED\n")
		} else {
			fmt.Printf("FAILED\n")
		}
	}

	return err
}

// Convert RunTTFTask to struct
type TTFTask struct{}

func (TTFTask) Run(prj *Project, fields map[string]any) error {
	source_fn := ""
	target_fn := ""

	var err error
	for k, v := range fields {
		switch k {
		case "source":
			if s, ok := v.(string); ok && s != "" {
				source_fn, err = prj.AbsPath(s)
				if err != nil {
					return fmt.Errorf("%s: %w", k, err)
				}
			} else {
				return fmt.Errorf("source: must be a non-empty string")
			}

		case "target":
			if s, ok := v.(string); ok && s != "" {
				target_fn, err = prj.AbsPath(s)
				if err != nil {
					return fmt.Errorf("%s: %w", k, err)
				}
			} else {
				return fmt.Errorf("%s: must be a non-empty string", k)
			}
		default:
			fmt.Printf("- WARNING: unknown field '%s'\n", k)
		}
	}

	if source_fn == "" {
		return fmt.Errorf("missing field: source")
	}
	if target_fn == "" {
		return fmt.Errorf("missing field: target")
	}

	if prj.Verbose {
		fmt.Printf("- source %q\n", source_fn)
		fmt.Printf("- target %q\n", target_fn)
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
	cmd = exec.Command("svg2ttf", source_fn, target_fn)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("svg2ttf: %w", err)
	}

	return nil
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

func readSVGFileAsGlyph(fn string) (*Glyph, error) {
	data, err := os.ReadFile(fn)
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
		w, u1, e1 := sg.Width.AsNumeric()
		h, u2, e2 := sg.Height.AsNumeric()
		if e1 == nil && e2 == nil &&
			(u1 == svg.UnitNone || u1 == svg.UnitPX) &&
			(u2 == svg.UnitNone || u2 == svg.UnitPX) {
			vb = &svg.ViewBoxValue{
				Width:  w,
				Height: h,
			}
		} else {
			return nil, fmt.Errorf("bad svg.viewBox attribute: %s", err)
		}
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

	g.FilePath = fn
	g.Width = w
	g.Height = h

	if len(sg.Items) == 0 {
		g.Path = ""
		g.Transform = nil
		return g, nil
	}

	switch v := sg.Items[0].(type) {
	case *svg.Path:
		_, err = svg.ParsePath(g.Path)
		if err != nil {
			return nil, err
		}
		g.Path = v.D
		g.Transform = v.Transform

	case *svg.Polygon:
		pp, err := svg.ParsePoints(v.Points)
		if err != nil {
			return nil, err
		}
		if len(pp) < 2 {
			return nil, fmt.Errorf("empty polygon data")
		}
		d := svg.PathData{}
		d.MoveTo(pp[0])
		for _, p := range pp[1:] {
			d.LineTo(p)
		}
		d.Close()
		g.Path = d.String()
		g.Transform = v.Transform

	case *svg.Circle:
		cx, err := lengthPixels(v.Cx, &g.Width)
		if err != nil {
			return nil, err
		}
		cy, err := lengthPixels(v.Cy, &g.Height)
		if err != nil {
			return nil, err
		}
		r := 1.0
		if v.Radius != "" {
			r, err = lengthPixels(v.Radius, &g.Width)
			if err != nil {
				return nil, err
			}
		}
		k := 0.551784 * r
		d := svg.PathData{}
		d.MoveTo(svg.Vertex{X: cx - r, Y: cy})
		d.CurveTo(
			svg.Vertex{X: cx - r, Y: cy - k},
			svg.Vertex{X: cx - k, Y: cy - r},
			svg.Vertex{X: cx, Y: cy - r})
		d.CurveTo(
			svg.Vertex{X: cx + k, Y: cy - r},
			svg.Vertex{X: cx + r, Y: cy - k},
			svg.Vertex{X: cx + r, Y: cy})
		d.CurveTo(
			svg.Vertex{X: cx + r, Y: cy + k},
			svg.Vertex{X: cx + k, Y: cy + r},
			svg.Vertex{X: cx, Y: cy + r})
		d.CurveTo(
			svg.Vertex{X: cx - k, Y: cy + r},
			svg.Vertex{X: cx - r, Y: cy + k},
			svg.Vertex{X: cx - r, Y: cy})
		d.Close()
		g.Path = d.String()
		g.Transform = v.Transform

	case *svg.Ellipse:
		cx, err := lengthPixels(v.Cx, &g.Width)
		if err != nil {
			return nil, err
		}
		cy, err := lengthPixels(v.Cy, &g.Height)
		if err != nil {
			return nil, err
		}
		rx, ry := 0.0, 0.0
		if v.Rx != "" {
			rx, err = lengthPixels(v.Rx, &g.Width)
			if err != nil {
				return nil, err
			}
			if v.Ry == "" {
				ry = rx
			}
		}
		if v.Ry != "" {
			ry, err = lengthPixels(v.Ry, &g.Height)
			if err != nil {
				return nil, err
			}
			if v.Rx == "" {
				rx = ry
			}
		}

		kx := 0.551784 * rx
		ky := 0.551784 * ry
		d := svg.PathData{}
		d.MoveTo(svg.Vertex{X: cx - rx, Y: cy})
		d.CurveTo(
			svg.Vertex{X: cx - rx, Y: cy - ky},
			svg.Vertex{X: cx - kx, Y: cy - ry},
			svg.Vertex{X: cx, Y: cy - ry})
		d.CurveTo(
			svg.Vertex{X: cx + kx, Y: cy - ry},
			svg.Vertex{X: cx + rx, Y: cy - ky},
			svg.Vertex{X: cx + rx, Y: cy})
		d.CurveTo(
			svg.Vertex{X: cx + rx, Y: cy + ky},
			svg.Vertex{X: cx + kx, Y: cy + ry},
			svg.Vertex{X: cx, Y: cy + ry})
		d.CurveTo(
			svg.Vertex{X: cx - kx, Y: cy + ry},
			svg.Vertex{X: cx - rx, Y: cy + ky},
			svg.Vertex{X: cx - rx, Y: cy})
		d.Close()
		g.Path = d.String()
		g.Transform = v.Transform

	case *svg.Rect:
		x, err := lengthPixels(v.X, &g.Width)
		if err != nil {
			return nil, err
		}
		y, err := lengthPixels(v.Y, &g.Height)
		if err != nil {
			return nil, err
		}
		width, err := lengthPixels(v.Width, &g.Width)
		if err != nil {
			return nil, err
		}
		height, err := lengthPixels(v.Height, &g.Height)
		if err != nil {
			return nil, err
		}

		rx, ry := 0.0, 0.0
		if v.Rx != "" {
			rx, err = lengthPixels(v.Rx, &g.Width)
			if err != nil {
				return nil, err
			}
			if v.Ry == "" {
				ry = rx
			}
		}
		if v.Ry != "" {
			ry, err = lengthPixels(v.Ry, &g.Height)
			if err != nil {
				return nil, err
			}
			if v.Rx == "" {
				rx = ry
			}
		}

		d := svg.PathData{}
		if rx <= 0 || ry <= 0 {
			d.MoveTo(svg.Vertex{X: x, Y: y})
			d.LineTo(svg.Vertex{X: x + width, Y: y})
			d.LineTo(svg.Vertex{X: x + width, Y: y + height})
			d.LineTo(svg.Vertex{X: x, Y: y + height})
			d.Close()
		} else {
			if rx > width*0.5 {
				rx = width * 0.5
			}
			if ry > height*0.5 {
				ry = height * 0.5
			}

			kx := (1.0 - 0.551784) * rx
			ky := (1.0 - 0.551784) * ry

			d.MoveTo(svg.Vertex{X: x + rx, Y: y})
			d.LineTo(svg.Vertex{X: x + width - rx, Y: y})
			d.LineTo(svg.Vertex{X: x + width - rx, Y: y})
			d.CurveTo(svg.Vertex{X: x + width - kx, Y: y}, svg.Vertex{X: x + width, Y: y + ky}, svg.Vertex{X: x + width, Y: y + ry})
			d.LineTo(svg.Vertex{X: x + width, Y: y + height - ry})
			d.CurveTo(svg.Vertex{X: x + width, Y: y - ky}, svg.Vertex{X: x + width - kx, Y: y + height}, svg.Vertex{X: x + width - rx, Y: y + height})
			d.LineTo(svg.Vertex{X: x + rx, Y: y + height})
			d.CurveTo(svg.Vertex{X: x + kx, Y: y + height}, svg.Vertex{X: x, Y: y + height - ky}, svg.Vertex{X: x, Y: y + height - ry})
			d.LineTo(svg.Vertex{X: x, Y: y + ry})
			d.CurveTo(svg.Vertex{X: x, Y: y + ky}, svg.Vertex{X: x + kx, Y: y}, svg.Vertex{X: x + rx, Y: y})
			d.Close()
		}
		d.Close()
		g.Path = d.String()
		g.Transform = v.Transform

	default:
		return nil, fmt.Errorf("unsupported SVG element")
	}

	return g, nil
}

func composeGlyphsIntoSVGFont(out io.Writer, glyphs []*Glyph,
	ascent, descent int, family string) error {

	height := ascent + descent
	horizAdvX := height

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
		family, horizAdvX,
		family,
		height, ascent, descent, horizAdvX)

	for _, g := range glyphs {
		pd, err := svg.ParsePath(g.Path)
		if err != nil {
			return err
		}

		scale := float64(height) / g.Height
		for i := range pd.Vertices {
			pd.Vertices[i].X = math.Round(pd.Vertices[i].X * scale)
			pd.Vertices[i].Y = math.Round((g.Height-pd.Vertices[i].Y)*scale - float64(descent))
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

type NamedCodepoint struct {
	Name    string `xml:"glyph-name,attr"`
	Unicode string `xml:"unicode,attr"`
}

func extractNamedCodepoints(source_fn string) (glyphs []*NamedCodepoint, err error) {
	var buf []byte
	buf, err = os.ReadFile(source_fn)
	if err != nil {
		return
	}

	// implemented: support for glyph name extraction from SVG font
	// todo: support for glyph name extraction from ttf/otf file

	type SVGFontLoader struct {
		XMLName xml.Name          `xml:"svg"`
		Glyphs  []*NamedCodepoint `xml:"defs>font>glyph"`
	}

	font := &SVGFontLoader{}
	err = xml.Unmarshal(buf, &font)
	if err != nil {
		return
	}

	return font.Glyphs, nil
}

func codegenGlyphNames(out io.Writer, glyphs []*NamedCodepoint, globalVars map[string]string, contentTemplate, entryTemplate string) error {
	first := true
	cpmin := rune(0)
	cpmax := cpmin
	for _, g := range glyphs {
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

	entryLines := []string{}
	for _, g := range glyphs {
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
		hex := fmt.Sprintf("%.4X", cp)
		ident_cpp := MakeCPPIdentStr(g.Name)

		entryVars := maps.Clone(globalVars)
		entryVars["name"] = g.Name
		entryVars["ident-cpp"] = ident_cpp
		entryVars["unicode"] = "U+" + hex
		entryVars["unicode-hex"] = hex
		entryVars["utf8"] = u8
		entryVars["utf8-escaped-cpp"] = escaped

		line, err := ExpandVariables(entryTemplate, entryVars)
		if err != nil {
			return fmt.Errorf("entry template: %w", err)
		}

		entryLines = append(entryLines, line)
	}

	globalVars["codepoint-min"] = fmt.Sprintf("%X", cpmin)
	globalVars["codepoint-max"] = fmt.Sprintf("%X", cpmax)
	globalVars["entries"] = strings.Join(entryLines, "\n")

	fileContent, err := ExpandVariables(contentTemplate, globalVars)
	if err != nil {
		return fmt.Errorf("hpp-template: %w", err)
	}

	_, err = out.Write([]byte(fileContent))
	return err
}

// parseCodepoint extracts unicode codepoint value from a string which can
// be a decimal number or a hex (U+0000 or 0x0000 or 0X0000)
func parseCodepoint(s string) (rune, error) {
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
		return 0, fmt.Errorf("invalid codepoint value %q: %w", s, err)
	}
	return rune(cp), nil
}

func lengthPixels(l svg.Length, reflength *float64) (float64, error) {
	v, u, err := l.AsNumeric()
	if err != nil {
		return 0, err
	}
	switch u {
	case svg.UnitNone, svg.UnitPX:
		return v, nil
	case svg.UnitPercent:
		if reflength == nil {
			return 0, fmt.Errorf("unsupported length percentage")
		} else {
			return *reflength * v / 100.0, nil
		}
	default:
		return 0, fmt.Errorf("unsupported length units")
	}
}
