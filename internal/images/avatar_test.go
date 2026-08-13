package images_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"toomanyhours-api/internal/images"
	"toomanyhours-api/internal/validate"
)

// pngOf builds a real PNG in memory, so the tests exercise a genuine decode
// rather than a fixture file nobody can inspect. The colour tracks x and y so
// that a cropped result and a squashed one differ in bytes.
func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return buf.Bytes()
}

// decodeBomb is an actual decode bomb: a valid PNG signature and IHDR chunk
// declaring an enormous image, and nothing after it. Forty-odd bytes on the
// wire that would allocate fourteen gigabytes if anything called Decode.
//
// Built by hand rather than encoded, because encoding one really would allocate
// fourteen gigabytes — which is the whole point of checking the header first.
func decodeBomb(w, h uint32) []byte {
	var ihdr bytes.Buffer
	ihdr.WriteString("IHDR")
	binary.Write(&ihdr, binary.BigEndian, w)
	binary.Write(&ihdr, binary.BigEndian, h)
	ihdr.Write([]byte{8, 6, 0, 0, 0}) // 8-bit RGBA, no compression/filter/interlace

	var out bytes.Buffer
	out.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	binary.Write(&out, binary.BigEndian, uint32(13)) // IHDR payload length
	out.Write(ihdr.Bytes())
	binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(ihdr.Bytes()))
	return out.Bytes()
}

func TestAvatarProducesA256SquareJPEG(t *testing.T) {
	out, err := images.Avatar(bytes.NewReader(pngOf(t, 800, 600)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("output does not decode: %v", err)
	}
	if format != "jpeg" {
		t.Errorf("format = %q, want jpeg", format)
	}
	if cfg.Width != 256 || cfg.Height != 256 {
		t.Errorf("size = %dx%d, want 256x256", cfg.Width, cfg.Height)
	}
}

// A PNG in, a JPEG out: the re-encode is what makes the stored bytes provably
// an image, so the output must not depend on the input's format.
func TestAvatarReencodesRatherThanPassingThrough(t *testing.T) {
	in := pngOf(t, 300, 300)

	out, err := images.Avatar(bytes.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bytes.Equal(in, out) {
		t.Error("output is byte-identical to the input; it was not re-encoded")
	}
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("output is not a JPEG: %v", err)
	}
}

func TestAvatarAcceptsNonSquareInput(t *testing.T) {
	for _, size := range [][2]int{{200, 800}, {800, 200}, {256, 256}, {50, 50}} {
		out, err := images.Avatar(bytes.NewReader(pngOf(t, size[0], size[1])))
		if err != nil {
			t.Fatalf("%dx%d: unexpected error: %v", size[0], size[1], err)
		}

		cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("%dx%d: output does not decode: %v", size[0], size[1], err)
		}
		if cfg.Width != 256 || cfg.Height != 256 {
			t.Errorf("%dx%d gave %dx%d, want 256x256", size[0], size[1], cfg.Width, cfg.Height)
		}
	}
}

// Dropping the centre-crop and scaling the whole rectangle passes every other
// assertion here — a squash is still a 256x256 JPEG, and two differently-shaped
// sources still produce different bytes either way. So this uses a fixture where
// the two answers differ in *content*: a 600x200 band, green only in the middle
// 200x200 square and red on both sides.
//
// A centred crop takes exactly the green band, so every pixel out is green. A
// squash takes the whole width, so the left of the output is red.
func TestAvatarCropsRatherThanSquashing(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 600, 200))
	for x := range 600 {
		for y := range 200 {
			if x >= 200 && x < 400 {
				img.Set(x, y, color.RGBA{G: 255, A: 255})
			} else {
				img.Set(x, y, color.RGBA{R: 255, A: 255})
			}
		}
	}
	var fixture bytes.Buffer
	if err := png.Encode(&fixture, img); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	out, err := images.Avatar(bytes.NewReader(fixture.Bytes()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("output does not decode: %v", err)
	}

	// Well inside the left edge, past any JPEG ringing at the boundary.
	r, g, _, _ := decoded.At(20, 128).RGBA()
	if r > g {
		t.Errorf("left edge is red (r=%d g=%d): the whole width was scaled in, not the centred square", r>>8, g>>8)
	}
}

func TestAvatarRejectsNonImages(t *testing.T) {
	_, err := images.Avatar(strings.NewReader("this is not an image, whatever the Content-Type said"))
	if !errors.Is(err, images.ErrNotAnImage) {
		t.Errorf("err = %v, want ErrNotAnImage", err)
	}
}

// The guard runs on the header, before Decode allocates anything — which is the
// only reason this test can exist at all.
func TestAvatarRejectsADecodeBomb(t *testing.T) {
	bomb := decodeBomb(60000, 60000)
	if len(bomb) > 100 {
		t.Fatalf("the bomb fixture is %d bytes; it should be a header and nothing else", len(bomb))
	}

	_, err := images.Avatar(bytes.NewReader(bomb))
	if !errors.Is(err, validate.ErrRange) {
		t.Errorf("err = %v, want ErrRange", err)
	}
}

func TestHashChangesWithContent(t *testing.T) {
	a := images.Hash([]byte("one"))
	b := images.Hash([]byte("two"))

	if a == b {
		t.Error("different bytes produced the same hash")
	}
	if len(a) != 12 {
		t.Errorf("hash length = %d, want 12", len(a))
	}
	if a != images.Hash([]byte("one")) {
		t.Error("the same bytes produced different hashes")
	}
}
