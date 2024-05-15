package tasks

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/adnsv/svg"
)

func RunVGConvertTask(prj *Project, fields map[string]any) error {
	sources := []string{}
	hpp_fn := ""
	cpp_fn := ""
	namespace := ""

	var err error

	for k, v := range fields {
		switch k {

		case "source":
			sources, err = prj.GetStrings(v)
			if err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}

		case "hpp-target":
			if s, ok := v.(string); ok && s != "" {
				hpp_fn, err = prj.AbsPath(s)
				if err != nil {
					return fmt.Errorf("%s: %w", k, err)
				}
			} else {
				return fmt.Errorf("%s: must be a non-empty string", k)
			}

		case "cpp-target":
			if s, ok := v.(string); ok && s != "" {
				cpp_fn, err = prj.AbsPath(s)
				if err != nil {
					return fmt.Errorf("%s: %w", k, err)
				}
			} else {
				return fmt.Errorf("%s: must be a non-empty string", k)
			}

		case "namespace":
			if s, ok := v.(string); ok && s != "" {
				namespace = s
			} else {
				return fmt.Errorf("%s: must be a non-empty string", k)
			}
		}
	}

	if hpp_fn == "" {
		return fmt.Errorf("missing field: hpp-target")
	}

	if cpp_fn == "" {
		return fmt.Errorf("missing field: cpp-target")
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

	hpp_buf := bytes.Buffer{}
	hpp_out := tabwriter.NewWriter(&hpp_buf, 0, 4, 4, ' ', 0)

	cpp_buf := bytes.Buffer{}
	cpp_out := tabwriter.NewWriter(&cpp_buf, 0, 4, 4, ' ', 0)

	inputs := []*VG{}
	for _, fn := range source_fns {
		vg, err := readSVGFile(fn)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", fn, err)
		}
		inputs = append(inputs, vg)
	}

	rel_name, _ := filepath.Rel(filepath.Dir(cpp_fn), hpp_fn)

	fmt.Fprintf(hpp_out, "#pragma once\n\n")
	fmt.Fprintf(hpp_out, "#include <array>\n\n")
	fmt.Fprintf(cpp_out, "#include %q\n\n", rel_name)

	if namespace != "" {
		fmt.Fprintf(hpp_out, "namespace %s {\n\n", namespace)
		fmt.Fprintf(cpp_out, "namespace %s {\n\n", namespace)
	}

	for _, vg := range inputs {
		writeVG(hpp_out, cpp_out, vg)
	}

	if namespace != "" {
		fmt.Fprintf(hpp_out, "} // namespace %s\n", namespace)
		fmt.Fprintf(cpp_out, "} // namespace %s\n", namespace)
	}

	hpp_out.Flush()
	cpp_out.Flush()

	fmt.Printf("- writing %s ... ", hpp_fn)
	err = os.WriteFile(hpp_fn, hpp_buf.Bytes(), 0666)
	if err == nil {
		fmt.Printf("SUCCEEDED\n")
	} else {
		fmt.Printf("FAILED\n")
	}

	fmt.Printf("- writing %s ... ", cpp_fn)
	err = os.WriteFile(cpp_fn, cpp_buf.Bytes(), 0666)
	if err == nil {
		fmt.Printf("SUCCEEDED\n")
	} else {
		fmt.Printf("FAILED\n")
	}

	return nil
}

type VG struct {
	Filename     string
	ViewBox      svg.ViewBoxValue
	Commands     string
	Vertices     []svg.Vector
	ColorIndices []int
	ColorValues  []RGBA
	Opacities    []float64
	Ids          []string
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

func (vg *VG) addColor(c RGBA) int {
	for i, v := range vg.ColorValues {
		if v == c {
			return i
		}
	}
	vg.ColorValues = append(vg.ColorValues, c)
	return len(vg.ColorValues) - 1
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
	vg.ColorIndices = append(vg.ColorIndices, vg.addColor(rgba))
}
func (vg *VG) StartLayer(opacity float64) {
	vg.Commands += "{"
	vg.Opacities = append(vg.Opacities, opacity)
}
func (vg *VG) StopLayer() {
	vg.Commands += "}"
}
func (vg *VG) PushID(id string) {
	vg.Commands += "["
	vg.Ids = append(vg.Ids, id)
}
func (vg *VG) PopID() {
	vg.Commands += "]"
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

func readGroup(vg *VG, g *svg.Group, xform *svg.Transform) error {
	if id := g.ID(); id != "" {
		vg.PushID(id)
		defer vg.PopID()
	}

	if g.Transform != nil {
		xform = svg.Concatenate(xform, g.Transform)
	}

	needsLayer := g.Opacity != nil && *g.Opacity < 1.0
	if needsLayer {
		vg.StartLayer(*g.Opacity)
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

	if needsLayer {
		vg.StopLayer()
	}

	return nil
}

func readRect(vg *VG, p *svg.Rect, xform *svg.Transform) error {
	if id := p.ID(); id != "" {
		vg.PushID(id)
		defer vg.PopID()
	}

	x, err := lengthPixels(p.X, &vg.ViewBox.Width)
	if err != nil {
		return err
	}
	y, err := lengthPixels(p.Y, &vg.ViewBox.Height)
	if err != nil {
		return err
	}
	width, err := lengthPixels(p.Width, &vg.ViewBox.Width)
	if err != nil {
		return err
	}
	height, err := lengthPixels(p.Height, &vg.ViewBox.Height)
	if err != nil {
		return err
	}

	rx, ry := 0.0, 0.0
	if p.Rx != "" {
		rx, err = lengthPixels(p.Rx, &width)
		if err != nil {
			return err
		}
		if p.Ry == "" {
			ry = rx
		}
	}
	if p.Ry != "" {
		ry, err = lengthPixels(p.Ry, &height)
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
	if id := p.ID(); id != "" {
		vg.PushID(id)
		defer vg.PopID()
	}

	cx, err := lengthPixels(p.Cx, &vg.ViewBox.Width)
	if err != nil {
		return err
	}
	cy, err := lengthPixels(p.Cy, &vg.ViewBox.Height)
	if err != nil {
		return err
	}
	r := 1.0
	if p.Radius != "" {
		r, err = lengthPixels(p.Radius, &vg.ViewBox.Width)
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
	if id := p.ID(); id != "" {
		vg.PushID(id)
		defer vg.PopID()
	}

	cx, err := lengthPixels(p.Cx, &vg.ViewBox.Width)
	if err != nil {
		return err
	}
	cy, err := lengthPixels(p.Cy, &vg.ViewBox.Height)
	if err != nil {
		return err
	}
	rx, ry := 0.0, 0.0
	if p.Rx != "" {
		rx, err = lengthPixels(p.Rx, &vg.ViewBox.Width)
		if err != nil {
			return err
		}
		if p.Ry == "" {
			ry = rx
		}
	}
	if p.Ry != "" {
		ry, err = lengthPixels(p.Ry, &vg.ViewBox.Height)
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

	if id := p.ID(); id != "" {
		vg.PushID(id)
		defer vg.PopID()
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

	if id := p.ID(); id != "" {
		vg.PushID(id)
		defer vg.PopID()
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

func packVG(src *VG) []byte {
	vertex_scale := float64(10)

	width_16 := uint16(src.ViewBox.Width * vertex_scale)
	height_16 := uint16(src.ViewBox.Height * vertex_scale)

	buf := []byte{}
	magic_ver := uint32(0xfff00001)
	block_tag := uint32(0xffee0000)

	start := func(block_id uint32, counter int) {
		buf = binary.LittleEndian.AppendUint32(buf, block_tag|block_id)
		buf = binary.LittleEndian.AppendUint32(buf, uint32(counter))
	}

	buf = binary.LittleEndian.AppendUint32(buf, magic_ver)
	buf = binary.LittleEndian.AppendUint16(buf, width_16)
	buf = binary.LittleEndian.AppendUint16(buf, height_16)

	if len(src.Commands) > 0 {
		start(1, len(src.Commands))
		buf = append(buf, []byte(src.Commands)...)
	}

	if len(src.Vertices) > 0 {
		start(2, len(src.Vertices))
		for _, v := range src.Vertices {
			x := int16((v.X - src.ViewBox.MinX) * vertex_scale)
			y := int16((v.Y - src.ViewBox.MinY) * vertex_scale)
			buf = binary.LittleEndian.AppendUint16(buf, uint16(x))
			buf = binary.LittleEndian.AppendUint16(buf, uint16(y))
		}
	}

	if len(src.ColorIndices) > 0 {
		start(3, len(src.ColorIndices))
		for _, v := range src.ColorIndices {
			buf = binary.LittleEndian.AppendUint16(buf, uint16(v))
		}
	}

	if len(src.ColorValues) > 0 {
		start(4, len(src.ColorValues))
		for _, v := range src.ColorValues {
			buf = append(buf, v.R, v.G, v.B, v.A)
		}
	}

	if len(src.Opacities) > 0 {
		start(5, len(src.Opacities))
		for _, v := range src.Opacities {
			if v < 0.0 {
				v = 0
			} else if v > 1.0 {
				v = 1.0
			}
			buf = append(buf, uint8(v*255))
		}
	}

	if len(src.Ids) > 0 {
		start(6, len(src.Ids))
		s := ""
		cur := 0
		for _, v := range src.Ids {
			cur += len(v)
			s += v
			buf = binary.LittleEndian.AppendUint16(buf, uint16(cur))
		}
		start(7, len(s))
		buf = append(buf, []byte(s)...)
	}

	// eof
	buf = binary.LittleEndian.AppendUint32(buf, block_tag)

	return buf
}

func writeVG(hpp, cpp io.Writer, src *VG) {
	buf := packVG(src)

	ident := MakeCPPIdentStr(strings.ToLower(RemoveExtension(filepath.Base(src.Filename))))

	bb := []string{}
	for _, v := range buf {
		bb = append(bb, fmt.Sprintf("0x%.2x", v))
	}

	fmt.Fprintf(hpp, "extern const std::array<unsigned char, %d> %s;\n\n",
		len(buf), ident)
	fmt.Fprintf(cpp, "const std::array<unsigned char, %d> %s = {\n%s\n};\n\n",
		len(buf), ident, CommaWrap(bb, "\t", 100))
}

func calcShapePaint(s *svg.Shape) RGBA {
	rgba := RGBA{
		R: 0,
		G: 0,
		B: 0,
		A: 255,
	}
	if s.Fill != nil {
		if s.Fill.Kind == svg.PaintKindRGB {
			rgba.R = s.Fill.Color.R
			rgba.G = s.Fill.Color.G
			rgba.B = s.Fill.Color.B
		}
	}

	if s.FillOpacity != nil {
		v := *s.FillOpacity
		if s.Opacity != nil {
			v = v * *s.Opacity
		}
		if v < 0.0 {
			rgba.A = 0
		} else if v < 1.0 {
			rgba.A = uint8(v * 255)
		}
	} else if s.Opacity != nil {
		v := *s.Opacity
		if v < 0.0 {
			rgba.A = 0
		} else if v < 1.0 {
			rgba.A = uint8(v * 255)
		}
	}

	return rgba
}
