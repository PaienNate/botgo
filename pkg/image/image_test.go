package image

import (
	"bytes"
	stdimage "image"
	"image/color"
	"image/png"
	"testing"

	"golang.org/x/image/bmp"
)

func testImage() *stdimage.RGBA {
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	return img
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, testImage()); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestIsGIFOrPNGOrJPG(t *testing.T) {
	if !IsGIFOrPNGOrJPG(testPNG(t)) {
		t.Fatal("png should be supported")
	}
	if IsGIFOrPNGOrJPG([]byte("not an image")) {
		t.Fatal("plain text should not be supported")
	}
}

func TestEncodeImagePassthrough(t *testing.T) {
	pngData := testPNG(t)
	out, err := EncodeImage(pngData)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, pngData) {
		t.Fatal("png should be returned as-is")
	}
}

func TestEncodeImageConvertsBMP(t *testing.T) {
	var bmpBuf bytes.Buffer
	if err := bmp.Encode(&bmpBuf, testImage()); err != nil {
		t.Fatal(err)
	}
	out, err := EncodeImage(bmpBuf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte("\xFF\xD8")) {
		t.Fatalf("bmp should be converted to jpeg, got prefix %x", out[:func() int {
			if len(out) < 4 {
				return len(out)
			}
			return 4
		}()])
	}
}
