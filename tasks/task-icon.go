package tasks

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
)

func init() {
	image.RegisterFormat("png", "png", png.Decode, png.DecodeConfig)
	image.RegisterFormat("jpeg", "jpeg", jpeg.Decode, jpeg.DecodeConfig)
}

type pixmapEntry struct {
	filename string
	ident    string
	size     image.Point
	img      image.Image
	frmt     string
}

func (pm *pixmapEntry) convert(dstfmt string) ([]byte, error) {
	switch dstfmt {
	case "png":
		{
			buf := &bytes.Buffer{}
			err := png.Encode(buf, pm.img)
			if err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}

	case "prgba": // premultiplied rgba
		{
			bounds := pm.img.Bounds()
			conv := image.NewRGBA(bounds)
			cm := conv.ColorModel()
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					s := pm.img.At(x, y)
					conv.Set(x, y, cm.Convert(s))
				}
			}
			return []byte(conv.Pix), nil
		}

	case "nrgba":
		{
			bounds := pm.img.Bounds()
			conv := image.NewNRGBA(bounds)
			cm := conv.ColorModel()
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					s := pm.img.At(x, y)
					conv.Set(x, y, cm.Convert(s))
				}
			}
			return []byte(conv.Pix), nil
		}

	default:
		return nil, fmt.Errorf("unsupported target pixmap format: %s", dstfmt)
	}
}

func loadPixmap(source_fn string, ident string) (*pixmapEntry, error) {
	binary, err := os.ReadFile(source_fn)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(binary)
	img, frmt, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()

	ret := &pixmapEntry{
		filename: filepath.Base(source_fn),
		ident:    ident,
		size:     bounds.Size(),
		frmt:     frmt,
		img:      img,
	}

	return ret, nil
}

func loadPixmaps(source_fns []string) ([]*pixmapEntry, error) {
	pixmaps := []*pixmapEntry{}
	for _, fn := range source_fns {
		name := filepath.Base(fn)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		name = strings.ToLower(name)
		name = strings.ReplaceAll(name, "-", "_")
		p, err := loadPixmap(fn, name)
		if err != nil {
			return nil, err
		}
		pixmaps = append(pixmaps, p)
	}
	sort.SliceStable(pixmaps, func(i, j int) bool {
		return naturalCompare(pixmaps[i].ident, pixmaps[j].ident) < 0
	})
	return pixmaps, nil
}

// Convert RunEmbedIconTask to struct
type EmbedIconTask struct{}

func (EmbedIconTask) Run(prj *Project, fields map[string]any) error {
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
		default:
			fmt.Printf("- WARNING: unknown field '%s'\n", k)
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

	pixmaps, err := loadPixmaps(source_fns)
	if err != nil {
		return err
	}

	buf := bytes.Buffer{}
	out := tabwriter.NewWriter(&buf, 0, 4, 1, ' ', 0)
	err = codegenGLFWIcon(out, pixmaps)
	if err != nil {
		return err
	}
	out.Flush()
	fmt.Printf("- writing %s ... ", target_fn)
	err = os.WriteFile(target_fn, buf.Bytes(), 0666)
	if err == nil {
		fmt.Printf("SUCCEEDED\n")
	} else {
		fmt.Printf("FAILED\n")
	}
	return nil
}

func codegenGLFWIcon(w io.Writer, pixmaps []*pixmapEntry) error {

	fmt.Fprintf(w, `// generated file, do not edit
#if defined(APPICON_GLFW)
#include <GLFW/glfw3.h>
#elif defined(APPICON_SDL2)
#include <SDL.h>
#elif defined(APPICON_SDL3)
#include <SDL3/SDL.h>
#endif

`)

	if len(pixmaps) == 0 {
		return nil
	}

	largestPixmap := pixmaps[0]

	for _, pixmap := range pixmaps {
		bb, err := pixmap.convert("nrgba")
		if err != nil {
			return err
		}

		if largestPixmap == nil || pixmap.size.Y > largestPixmap.size.Y {
			largestPixmap = pixmap
		}

		s := "   "
		for i, b := range bb {
			if i > 0 && i%32 == 0 {
				s += "\n    "
			}
			s += fmt.Sprintf("0x%.2x,", b)
		}
		fmt.Fprintf(w, "unsigned char const %s[%d] = {\n", pixmap.ident, len(bb))
		fmt.Fprintln(w, s)
		fmt.Fprintf(w, "};\n\n")
	}

	fmt.Fprint(w, "extern void setup_app_icon(void* native_window_handle)\n{\n")

	{
		fmt.Fprint(w, "#if defined(APPICON_GLFW)\n")
		fmt.Fprintf(w, "    static GLFWimage const images[%d] = {\n", len(pixmaps))
		for _, pixmap := range pixmaps {
			fmt.Fprintf(w, "        {%d, %d, const_cast<unsigned char*>(%s)},\n", pixmap.size.X, pixmap.size.Y, pixmap.ident)
		}
		fmt.Fprintf(w, "    };\n")
		fmt.Fprintf(w, "    glfwSetWindowIcon(static_cast<GLFWwindow*>(native_window_handle), %d, images);\n\n", len(pixmaps))
	}

	{
		fmt.Fprintf(w, `#elif defined(APPICON_SDL2)
    auto surface = SDL_CreateRGBSurfaceWithFormatFrom(
        const_cast<void*>(reinterpret_cast<void const*>(%s)),
        %d, %d, 32, %d * 4, SDL_PIXELFORMAT_ABGR8888);
    SDL_SetWindowIcon(static_cast<SDL_Window*>(native_window_handle), surface);
    SDL_FreeSurface(surface);

`, largestPixmap.ident, largestPixmap.size.X, largestPixmap.size.Y, largestPixmap.size.X)
	}

	{
		fmt.Fprintf(w, `#elif defined(APPICON_SDL3)
    auto surface = SDL_CreateSurfaceFrom(%d, %d, SDL_PIXELFORMAT_ABGR8888, 
        const_cast<void*>(reinterpret_cast<void const*>(%s)), %d * 4);
    SDL_SetWindowIcon(static_cast<SDL_Window*>(native_window_handle), surface);
    SDL_DestroySurface(surface);

`, largestPixmap.size.X, largestPixmap.size.Y, largestPixmap.ident, largestPixmap.size.X)
	}

	fmt.Fprint(w, "#endif\n")

	fmt.Fprintf(w, "};\n")

	return nil
}

// Convert RunWin32IconTask to struct
type Win32IconTask struct{}

func (Win32IconTask) Run(prj *Project, fields map[string]any) error {
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

		default:
			fmt.Printf("- WARNING: unknown field '%s'\n", k)
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

	pixmaps, err := loadPixmaps(source_fns)
	if err != nil {
		return err
	}

	buf, err := produceWin32Icon(pixmaps)
	if err != nil {
		return err
	}

	fmt.Printf("- writing %s ... ", target_fn)
	err = os.WriteFile(target_fn, buf, 0666)
	if err == nil {
		fmt.Printf("SUCCEEDED\n")
	} else {
		fmt.Printf("FAILED\n")
	}
	return nil
}

func produceWin32Icon(pixmaps []*pixmapEntry) ([]byte, error) {
	makedim := func(sz int) uint8 {
		if sz == 256 {
			return uint8(0)
		}
		return uint8(sz)
	}

	aligned := func(sz int) int {
		return (sz + 3) & ^3
	}

	calcPadding := func(sz int) int {
		return aligned(sz) - sz
	}

	buf := &bytes.Buffer{}
	binary.Write(buf, binary.LittleEndian, uint16(0)) // reserved, always 0
	binary.Write(buf, binary.LittleEndian, uint16(1)) // image type: icon
	binary.Write(buf, binary.LittleEndian, uint16(len(pixmaps)))

	dirEntrySize := 16
	headerSize := 6 + len(pixmaps)*dirEntrySize
	imageOffset := headerSize

	// image header
	blobs := [][]byte{}
	for _, pixmap := range pixmaps {

		png, err := pixmap.convert("png")
		if err != nil {
			return nil, err
		}
		blobs = append(blobs, png)

		binary.Write(buf, binary.LittleEndian, makedim(pixmap.size.X))
		binary.Write(buf, binary.LittleEndian, makedim(pixmap.size.Y))
		binary.Write(buf, binary.LittleEndian, uint8(0))   // color count
		binary.Write(buf, binary.LittleEndian, uint8(0))   // reserved, always 0
		binary.Write(buf, binary.LittleEndian, uint16(1))  // color planes
		binary.Write(buf, binary.LittleEndian, uint16(32)) //bits per pixel
		binary.Write(buf, binary.LittleEndian, uint32(len(png)))
		binary.Write(buf, binary.LittleEndian, uint32(imageOffset))
		imageOffset = imageOffset + aligned(len(png))
	}
	for _, blob := range blobs {
		binary.Write(buf, binary.LittleEndian, blob)
		pad := calcPadding(len(blob))
		for pad > 0 {
			binary.Write(buf, binary.LittleEndian, uint8(0))
			pad--
		}
	}

	return buf.Bytes(), nil
}
