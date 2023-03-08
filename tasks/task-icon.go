package tasks

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adnsv/btr/codegen"
)

type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

func RunIcon(task *Task, config *Project, iconType string) error {
	var err error

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
	filepaths, err := AbsExistingPaths(config.BaseDir, sources)
	if err != nil {
		return err
	}

	images := []*imageEntry{}
	maxPathLength := 0
	for _, fn := range filepaths {
		if len(fn) > maxPathLength {
			maxPathLength = len(fn)
		}
	}
	for _, path := range filepaths {
		name := filepath.Base(path)
		name = name[:len(name)-len(filepath.Ext(path))]
		name = strings.ReplaceAll(name, "-", "_")

		n := maxPathLength - len(path) + 1
		if n < 1 {
			n = 1
		}
		if config.Verbose {
			fmt.Printf("loading %q\n", path)
		}
		img, err := loadImage(path, name, "png")
		if err != nil {
			return err
		}
		if img.width > 256 || img.height > 256 || img.width == 0 || img.height == 0 {
			continue
		}
		/*
			bh := BITMAPINFOHEADER{}
			bh.BiSize = 40
			bh.BiWidth = int32(img.width)
			bh.BiHeight = -int32(img.height)
			bh.BiPlanes = 1
			bh.BiBitCount = 32
			bh.BiCompression = 0x0005 // BI_PNG
			bh.BiSizeImage = uint32(len(img.data))
			bh.BiXPelsPerMeter = 0
			bh.BiYPelsPerMeter = 0
			bh.BiClrUsed = 0
			bh.BiClrImportant = 0
			buf := bytes.Buffer{}
			binary.Write(&buf, binary.LittleEndian, &bh)
			binary.Write(&buf, binary.LittleEndian, img.data)
			img.data = buf.Bytes()
		*/

		images = append(images, img)
	}

	sort.SliceStable(images, func(i, j int) bool {
		return strings.Compare(images[i].name, images[j].name) < 0
	})

	buf := &bytes.Buffer{}
	binary.Write(buf, binary.LittleEndian, uint16(0)) // reserved, always 0
	binary.Write(buf, binary.LittleEndian, uint16(1)) // image type: icon
	binary.Write(buf, binary.LittleEndian, uint16(len(images)))

	dirEntrySize := 16
	headerSize := 6 + len(images)*dirEntrySize
	imageOffset := headerSize

	for _, img := range images {
		// image header
		makedim := func(sz int) uint8 {
			if sz == 256 {
				return uint8(0)
			}
			return uint8(sz)
		}
		binary.Write(buf, binary.LittleEndian, makedim(img.width))
		binary.Write(buf, binary.LittleEndian, makedim(img.height))
		binary.Write(buf, binary.LittleEndian, uint8(0))   // color count
		binary.Write(buf, binary.LittleEndian, uint8(0))   // reserved, always 0
		binary.Write(buf, binary.LittleEndian, uint16(1))  // color planes
		binary.Write(buf, binary.LittleEndian, uint16(32)) //bits per pixel
		binary.Write(buf, binary.LittleEndian, uint32(len(img.data)))
		binary.Write(buf, binary.LittleEndian, uint32(imageOffset))
		imageOffset = imageOffset + aligned(len(img.data))
	}
	for _, img := range images {
		binary.Write(buf, binary.LittleEndian, img.data)
		pad := calcPadding(len(img.data))
		for pad > 0 {
			binary.Write(buf, binary.LittleEndian, uint8(0))
			pad--
		}
	}

	out := codegen.NewBuffer(dst, config.Codegen)
	out.Write(buf.Bytes())
	return out.WriteOut()
}

func aligned(sz int) int {
	return (sz + 3) & ^3
}

func calcPadding(sz int) int {
	return aligned(sz) - sz
}
