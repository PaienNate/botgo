// Package image 提供 QQ 官方富媒体所需的基础图片格式适配。
//
// 只处理 QQ 官方明确支持的图片格式：png / jpg / gif。
// 非支持格式会尽量转成 png（bmp 转 jpg），不依赖外部 ffmpeg。
package image

import (
	"bytes"
	"errors"
	"fmt"
	stdimage "image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"

	"github.com/disintegration/imaging"
)

const (
	// MaxImageBytes 单张图片最大输入大小，默认 50 MiB。
	MaxImageBytes = 50 << 20
	// MaxImageDimension 单边最大像素，防止解压炸弹。
	MaxImageDimension = 10000
)

const (
	headerJPG   = "\xFF\xD8"
	headerPNG   = "\x89PNG\r\n\x1a\n"
	headerGIF87 = "GIF87a"
	headerGIF89 = "GIF89a"
	scanLimit   = 4 * 1024
)

// IsGIFOrPNGOrJPG 判断数据是否已经是 QQ 官方支持的图片格式。
func IsGIFOrPNGOrJPG(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	if bytes.HasPrefix(data, []byte(headerGIF87)) || bytes.HasPrefix(data, []byte(headerGIF89)) {
		return true
	}
	if bytes.HasPrefix(data, []byte(headerPNG)) {
		return true
	}
	return bytes.HasPrefix(data, []byte(headerJPG))
}

// CheckImage 通过 content-type 判断数据是否像图片。
func CheckImage(r io.ReadSeeker) (string, bool) {
	contentType := scanType(r)
	return contentType, len(contentType) >= 6 && contentType[:6] == "image/"
}

func scanType(r io.ReadSeeker) string {
	_, _ = r.Seek(0, io.SeekStart)
	defer func() { _, _ = r.Seek(0, io.SeekStart) }()
	buf := make([]byte, scanLimit)
	n, _ := r.Read(buf)
	return http.DetectContentType(buf[:n])
}

// EncodeImage 把非 png/jpg/gif 的图片转成 QQ 官方支持的格式。
// 已经是 png/jpg/gif 的数据会原样返回。
func EncodeImage(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty image data")
	}
	if len(data) > MaxImageBytes {
		return nil, fmt.Errorf("image too large: %d bytes (max %d)", len(data), MaxImageBytes)
	}
	if IsGIFOrPNGOrJPG(data) {
		return data, nil
	}

	cfg, format, err := stdimage.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image config: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, fmt.Errorf("invalid image size: %dx%d", cfg.Width, cfg.Height)
	}
	if cfg.Width > MaxImageDimension || cfg.Height > MaxImageDimension {
		return nil, fmt.Errorf("image dimension too large: %dx%d (max %d)", cfg.Width, cfg.Height, MaxImageDimension)
	}

	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	var buf bytes.Buffer
	if format == "bmp" {
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return nil, fmt.Errorf("encode jpeg: %w", err)
		}
		return buf.Bytes(), nil
	}
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}
