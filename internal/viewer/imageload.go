package viewer

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

const (
	imagePixelLimit    int64 = 100_000_000
	imageFileLimit     int64 = 512 << 20
	animatedBytesLimit int64 = 256 << 20
	bytesPerPixel      int64 = 4
	megapixel          int64 = 1_000_000
	sipsTool                 = "sips"
	sipsFormatFlag           = "-s"
	sipsFormatKey            = "format"
	sipsFormatPNG            = "png"
	sipsOutFlag              = "--out"
	sipsGetFlag              = "-g"
	sipsTempPattern          = "viewer-*.png"
	sipsPixelWidthKey        = "pixelWidth"
	sipsPixelHeightKey       = "pixelHeight"
)

type tooLargeError struct {
	width, height int
	limit         int64
}

func (e *tooLargeError) Error() string {
	return fmt.Sprintf("image is %d x %d px (%d megapixels); the limit is %d megapixels",
		e.width, e.height, int64(e.width)*int64(e.height)/megapixel, e.limit/megapixel)
}

type decoded struct {
	bitmap      *ui.Bitmap
	svg         *ui.SVG
	frames      ui.VideoDecoder
	width       int
	height      int
	format      string
	unsupported []string
}

type converter func(path string, limit int64) ([]byte, error)

// TECHDEBT: The whole image file is read into memory, up to imageFileLimit; stream the decode if larger files matter.
func readLimited(path string, limit int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("file is %s; the limit is %s", formatSize(info.Size()), formatSize(limit))
	}
	return os.ReadFile(path)
}

func decodeSVG(data []byte) (*decoded, error) {
	svg, err := ui.ParseSVG(data)
	if err != nil {
		return nil, fmt.Errorf("parse SVG: %w", err)
	}
	w, h := svg.Size()
	return &decoded{svg: svg, width: int(math.Round(float64(w))), height: int(math.Round(float64(h))), format: "SVG", unsupported: svg.Unsupported()}, nil
}

func decodeRaster(data []byte, path string, limit int64, convert converter) (*decoded, error) {
	format := ""
	cfg, name, err := image.DecodeConfig(bytes.NewReader(data))
	switch {
	case err == nil:
		format = strings.ToUpper(name)
	case errors.Is(err, image.ErrFormat) && convert != nil:
		if data, err = convert(path, limit); err != nil {
			return nil, err
		}
		format = strings.ToUpper(strings.TrimPrefix(filepath.Ext(path), "."))
		if cfg, _, err = image.DecodeConfig(bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("decode converted image: %w", err)
		}
	default:
		return nil, fmt.Errorf("decode image: %w", err)
	}
	if int64(cfg.Width)*int64(cfg.Height) > limit {
		return nil, &tooLargeError{width: cfg.Width, height: cfg.Height, limit: limit}
	}
	if frames := animatedFrames(data, cfg); frames != nil {
		return &decoded{frames: frames, width: cfg.Width, height: cfg.Height, format: format}, nil
	}
	bm, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	w, h := bm.Size()
	bm.Pixelated()
	return &decoded{bitmap: bm, width: w, height: h, format: format}, nil
}

func animatedFrames(data []byte, cfg image.Config) ui.VideoDecoder {
	pixels := int64(cfg.Width) * int64(cfg.Height) * bytesPerPixel
	switch {
	case gifFrameCount(data) > 1:
		if pixels*int64(gifFrameCount(data)) > animatedBytesLimit {
			return nil
		}
		d, err := ui.NewGIFDecoder(data)
		if err != nil {
			return nil
		}
		return d
	case isAPNG(data):
		d, err := ui.NewAPNGDecoder(data)
		if err != nil {
			return nil
		}
		return d
	}
	return nil
}

func sipsConvert(path string, limit int64) ([]byte, error) {
	width, height, err := sipsSize(path)
	if err != nil {
		return nil, err
	}
	if int64(width)*int64(height) > limit {
		return nil, &tooLargeError{width: width, height: height, limit: limit}
	}
	tmp, err := os.CreateTemp("", sipsTempPattern)
	if err != nil {
		return nil, fmt.Errorf("convert image: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("convert image: %w", err)
	}
	out, err := exec.Command(sipsTool, sipsFormatFlag, sipsFormatKey, sipsFormatPNG, path, sipsOutFlag, name).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("sips: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return readLimited(name, imageFileLimit)
}

func sipsSize(path string) (int, int, error) {
	out, err := exec.Command(sipsTool, sipsGetFlag, sipsPixelWidthKey, sipsGetFlag, sipsPixelHeightKey, path).CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("sips: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return parseSipsSize(string(out))
}

func parseSipsSize(out string) (int, int, error) {
	values := map[string]int{}
	for _, line := range strings.Split(out, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			values[key] = n
		}
	}
	w, h := values[sipsPixelWidthKey], values[sipsPixelHeightKey]
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("sips reported no image size")
	}
	return w, h, nil
}
