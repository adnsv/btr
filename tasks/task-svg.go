package tasks

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/adnsv/svg"
)

func RunSVGConvertTask(prj *Project, fields map[string]any) error {
	sources := []string{}
	target_fn := ""

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

		}
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

	inputs := []*VG{}

	for _, fn := range source_fns {
		vg, err := readSVGFile(fn)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", fn, err)
		}
		inputs = append(inputs, vg)
	}

	buf := bytes.Buffer{}
	out := tabwriter.NewWriter(&buf, 0, 4, 1, ' ', 0)

	fmt.Fprintf(out, "// vector paths\n")

	for _, vg := range inputs {
		writeVG(out, vg)
	}

	fmt.Printf("- writing %s ... ", target_fn)
	out.Flush()
	err = os.WriteFile(target_fn, buf.Bytes(), 0666)
	if err == nil {
		fmt.Printf("SUCCEEDED\n")
	} else {
		fmt.Printf("FAILED\n")
	}

	return nil
}

type VG struct {
	Filename string
	ViewBox  svg.ViewBoxValue
	Commands string
	Vertices []svg.Vector
	Fills    []RGBA
}

type RGBA struct {
	R uint8
	G uint8
	B uint8
	A uint8
}

func addVertex(vg *VG, xform *svg.Transform, v svg.Vertex) {
	x, y := xform.CalcAbs(v.X, v.Y)
	vg.Vertices = append(vg.Vertices, svg.Vertex{X: x, Y: y})
}

func (vg *VG) Close() {
	vg.Commands += "z"
}

func (vg *VG) MoveTo(xform *svg.Transform, v svg.Vertex) {
	vg.Commands += "m"
	addVertex(vg, xform, v)
}
func (vg *VG) LineTo(xform *svg.Transform, v svg.Vertex) {
	vg.Commands += "l"
	addVertex(vg, xform, v)
}
func (vg *VG) CurveTo(xform *svg.Transform, c1, c2, v svg.Vertex) {
	vg.Commands += "c"
	addVertex(vg, xform, c1)
	addVertex(vg, xform, c2)
	addVertex(vg, xform, v)
}

func (vg *VG) Fill(rgba RGBA) {
	vg.Commands += "f"
	vg.Fills = append(vg.Fills, rgba)
}

func readSVGFile(fn string) (*VG, error) {
	data, err := os.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	sg, err := svg.Parse(string(data))
	if err != nil {
		return nil, err
	}
	vg := &VG{Filename: fn}

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

	vg.ViewBox = *vb

	xform := svg.UnitTransform()
	readGroup(vg, &sg.Group, xform)

	return vg, nil
}

func lengthPixels(vg *VG, l svg.Length, reflength *float64) (float64, error) {
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

func readGroup(vg *VG, g *svg.Group, xform *svg.Transform) error {

	if g.Transform != nil {
		xform = svg.Concatenate(xform, g.Transform)
	}
	for _, it := range g.Items {
		switch v := it.(type) {
		case *svg.Group:
			err := readGroup(vg, v, xform)
			if err != nil {
				return err
			}

		case *svg.Line:

		case *svg.Rect:
			err := readRect(vg, v, xform)
			if err != nil {
				return err
			}

		case *svg.Circle:
			err := readCircle(vg, v, xform)
			if err != nil {
				return err
			}

		case *svg.Ellipse:
			err := readEllipse(vg, v, xform)
			if err != nil {
				return err
			}

		case *svg.Polygon:
			err := readPolygon(vg, v, xform)
			if err != nil {
				return err
			}

		case *svg.Path:
			err := readPath(vg, v, xform)
			if err != nil {
				return err
			}

		default:
			return errors.New("unsupported element tag")
		}
	}

	return nil
}

func readRect(vg *VG, p *svg.Rect, xform *svg.Transform) error {
	x, err := lengthPixels(vg, p.X, &vg.ViewBox.Width)
	if err != nil {
		return err
	}
	y, err := lengthPixels(vg, p.Y, &vg.ViewBox.Height)
	if err != nil {
		return err
	}
	width, err := lengthPixels(vg, p.Width, &vg.ViewBox.Width)
	if err != nil {
		return err
	}
	height, err := lengthPixels(vg, p.Height, &vg.ViewBox.Height)
	if err != nil {
		return err
	}

	rx, ry := 0.0, 0.0
	if p.Rx != "" {
		rx, err = lengthPixels(vg, p.Rx, &width)
		if err != nil {
			return err
		}
		if p.Ry == "" {
			ry = rx
		}
	}
	if p.Ry != "" {
		ry, err = lengthPixels(vg, p.Ry, &height)
		if err != nil {
			return err
		}
		if p.Rx == "" {
			rx = ry
		}
	}

	if rx <= 0 || ry <= 0 {
		vg.MoveTo(xform, svg.Vertex{X: x, Y: y})
		vg.LineTo(xform, svg.Vertex{X: x + width, Y: y})
		vg.LineTo(xform, svg.Vertex{X: x + width, Y: y + height})
		vg.LineTo(xform, svg.Vertex{X: x, Y: y + height})
		vg.Close()
	} else {
		if rx > width*0.5 {
			rx = width * 0.5
		}
		if ry > height*0.5 {
			ry = height * 0.5
		}

		kx := (1.0 - 0.551784) * rx
		ky := (1.0 - 0.551784) * ry

		vg.MoveTo(xform, svg.Vertex{X: x + rx, Y: y})
		vg.LineTo(xform, svg.Vertex{X: x + width - rx, Y: y})
		vg.LineTo(xform, svg.Vertex{X: x + width - rx, Y: y})
		vg.CurveTo(xform, svg.Vertex{X: x + width - kx, Y: y}, svg.Vertex{X: x + width, Y: y + ky}, svg.Vertex{X: x + width, Y: y + ry})
		vg.LineTo(xform, svg.Vertex{X: x + width, Y: y + height - ry})
		vg.CurveTo(xform, svg.Vertex{X: x + width, Y: y - ky}, svg.Vertex{X: x + width - kx, Y: y + height}, svg.Vertex{X: x + width - rx, Y: y + height})
		vg.LineTo(xform, svg.Vertex{X: x + rx, Y: y + height})
		vg.CurveTo(xform, svg.Vertex{X: x + kx, Y: y + height}, svg.Vertex{X: x, Y: y + height - ky}, svg.Vertex{X: x, Y: y + height - ry})
		vg.LineTo(xform, svg.Vertex{X: x, Y: y + ry})
		vg.CurveTo(xform, svg.Vertex{X: x, Y: y + ky}, svg.Vertex{X: x + kx, Y: y}, svg.Vertex{X: x + rx, Y: y})
		vg.Close()
	}

	vg.Fill(calcShapePaint(&p.Shape))

	return nil
}

func readCircle(vg *VG, p *svg.Circle, xform *svg.Transform) error {
	cx, err := lengthPixels(vg, p.Cx, &vg.ViewBox.Width)
	if err != nil {
		return err
	}
	cy, err := lengthPixels(vg, p.Cy, &vg.ViewBox.Height)
	if err != nil {
		return err
	}
	r := 1.0
	if p.Radius != "" {
		r, err = lengthPixels(vg, p.Radius, &vg.ViewBox.Width)
		if err != nil {
			return err
		}
	}

	k := 0.551784 * r

	vg.MoveTo(xform, svg.Vertex{X: cx - r, Y: cy})
	vg.CurveTo(xform,
		svg.Vertex{X: cx - r, Y: cy - k},
		svg.Vertex{X: cx - k, Y: cy - r},
		svg.Vertex{X: cx, Y: cy - r})
	vg.CurveTo(xform,
		svg.Vertex{X: cx + k, Y: cy - r},
		svg.Vertex{X: cx + r, Y: cy - k},
		svg.Vertex{X: cx + r, Y: cy})
	vg.CurveTo(xform,
		svg.Vertex{X: cx + r, Y: cy + k},
		svg.Vertex{X: cx + k, Y: cy + r},
		svg.Vertex{X: cx, Y: cy + r})
	vg.CurveTo(xform,
		svg.Vertex{X: cx - k, Y: cy + r},
		svg.Vertex{X: cx - r, Y: cy + k},
		svg.Vertex{X: cx - r, Y: cy})
	vg.Close()

	vg.Fill(calcShapePaint(&p.Shape))
	return nil
}

func readEllipse(vg *VG, p *svg.Ellipse, xform *svg.Transform) error {
	cx, err := lengthPixels(vg, p.Cx, &vg.ViewBox.Width)
	if err != nil {
		return err
	}
	cy, err := lengthPixels(vg, p.Cy, &vg.ViewBox.Height)
	if err != nil {
		return err
	}
	rx, ry := 0.0, 0.0
	if p.Rx != "" {
		rx, err = lengthPixels(vg, p.Rx, &vg.ViewBox.Width)
		if err != nil {
			return err
		}
		if p.Ry == "" {
			ry = rx
		}
	}
	if p.Ry != "" {
		ry, err = lengthPixels(vg, p.Ry, &vg.ViewBox.Height)
		if err != nil {
			return err
		}
		if p.Rx == "" {
			rx = ry
		}
	}

	kx := 0.551784 * rx
	ky := 0.551784 * ry

	vg.MoveTo(xform, svg.Vertex{X: cx - rx, Y: cy})
	vg.CurveTo(xform,
		svg.Vertex{X: cx - rx, Y: cy - ky},
		svg.Vertex{X: cx - kx, Y: cy - ry},
		svg.Vertex{X: cx, Y: cy - ry})
	vg.CurveTo(xform,
		svg.Vertex{X: cx + kx, Y: cy - ry},
		svg.Vertex{X: cx + rx, Y: cy - ky},
		svg.Vertex{X: cx + rx, Y: cy})
	vg.CurveTo(xform,
		svg.Vertex{X: cx + rx, Y: cy + ky},
		svg.Vertex{X: cx + kx, Y: cy + ry},
		svg.Vertex{X: cx, Y: cy + ry})
	vg.CurveTo(xform,
		svg.Vertex{X: cx - kx, Y: cy + ry},
		svg.Vertex{X: cx - rx, Y: cy + ky},
		svg.Vertex{X: cx - rx, Y: cy})
	vg.Close()

	vg.Fill(calcShapePaint(&p.Shape))
	return nil
}

func readPolygon(vg *VG, p *svg.Polygon, xform *svg.Transform) error {
	pp, err := svg.ParsePoints(p.Points)
	if err != nil {
		return err
	}

	if len(pp) < 2 {
		return nil
	}

	vg.MoveTo(xform, pp[0])
	for _, p := range pp[1:] {
		vg.LineTo(xform, p)
	}
	vg.Close()
	vg.Fill(calcShapePaint(&p.Shape))

	return nil
}

func readPath(vg *VG, p *svg.Path, xform *svg.Transform) error {
	pp, err := svg.ParsePath(p.D)
	if err != nil {
		return err
	}

	vv := pp.Vertices
	for _, cmd := range pp.Commands {
		switch cmd {
		case svg.PathClose:
			vg.Close()

		case svg.PathMoveTo:
			if len(vv) < 1 {
				return errors.New("invalid # of vertices in path")
			}
			vg.MoveTo(xform, vv[0])
			vv = vv[1:]

		case svg.PathLineTo:

			if len(vv) < 1 {
				return errors.New("invalid # of vertices in path")
			}
			vg.LineTo(xform, vv[0])
			vv = vv[1:]

		case svg.PathCurveTo:
			if len(vv) < 3 {
				return errors.New("invalid # of vertices in path")
			}
			vg.CurveTo(xform, vv[0], vv[1], vv[2])
			vv = vv[3:]

		default:
			return errors.New("unsupported path command")
		}
	}

	vg.Fill(calcShapePaint(&p.Shape))

	return nil
}

func writeVG(out io.Writer, src *VG) {

	fmt.Fprintf(out, "// source: %s\n\n", filepath.Base(src.Filename))
	fmt.Fprintf(out, "#include <array>\n")
	fmt.Fprintf(out, "#include <string_view>\n\n")

	ident := MakeCPPIdentStr(strings.ToLower(RemoveExtension(filepath.Base(src.Filename))))

	fmt.Fprintf(out, "const std::string_view %s_commands = \n\t\"", ident)
	fmt.Fprintf(out, "%s", src.Commands)
	fmt.Fprintf(out, "\";\n\n")

	scale := float64(10)

	fmt.Fprintf(out, "const int %s_x = %d;\n", ident, int(src.ViewBox.MinX*scale))
	fmt.Fprintf(out, "const int %s_y = %d;\n", ident, int(src.ViewBox.MinY*scale))
	fmt.Fprintf(out, "const int %s_w = %d;\n", ident, int(src.ViewBox.Width*scale))
	fmt.Fprintf(out, "const int %s_h = %d;\n", ident, int(src.ViewBox.Height*scale))

	fmt.Fprintf(out, "const std::array<int16_t, %d> %s_vertices = {{\n", len(src.Vertices)*2, ident)

	s := "\t"
	for i, v := range src.Vertices {
		if i > 0 && i%8 == 0 {
			s += "\n\t"
		} else if i > 0 {
			s += " "
		}
		s += fmt.Sprintf("%d,%d,", int(v.X*scale), int(v.Y*scale))
	}
	fmt.Fprint(out, s)
	fmt.Fprintf(out, "\n}};\n\n")

	fmt.Fprintf(out, "const std::array<uint32_t, %d> %s_fills = {{\n", len(src.Fills), ident)
	s = "\t"
	for i, v := range src.Fills {
		if i > 0 && i%16 == 0 {
			s += "\n\t"
		} else if i > 0 {
			s += " "
		}
		s += fmt.Sprintf("0x%.2x%.2x%.2x%.2x,", v.A, v.B, v.G, v.R)
	}
	fmt.Fprint(out, s)
	fmt.Fprintf(out, "\n}};\n\n")

}

func calcShapePaint(s *svg.Shape) RGBA {
	rgba := RGBA{}
	if s.Fill.Kind == svg.PaintKindRGB {
		rgba.R = s.Fill.Color.R
		rgba.G = s.Fill.Color.G
		rgba.B = s.Fill.Color.B
	}
	rgba.A = 255
	if s.FillOpacity != nil {
		if *s.FillOpacity < 0.0 {
			rgba.A = 0
		} else if *s.FillOpacity < 1.0 {
			rgba.A = uint8(*s.FillOpacity * 255)
		}
	}
	return rgba
}
