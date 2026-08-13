// Package images turns an uploaded file into the bytes worth storing. Like
// validate, ratelimit, refresh and igdb it has no Gin and no GORM: it takes a
// reader and returns bytes.
package images

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"

	// Registers the decoders. An upload may be any of the three; what comes
	// out is always JPEG.
	_ "image/gif"
	_ "image/png"

	"golang.org/x/image/draw"

	"toomanyhours-api/internal/validate"
)

// ErrNotAnImage means the bytes did not decode. The Content-Type and filename a
// client sends are claims, not facts; this is the check that settles it.
var ErrNotAnImage = errors.New("images: not a decodable image")

const (
	// avatarSize covers an 80px circle on a 3x screen and costs about 20KB.
	avatarSize = 256
	// jpegQuality is where the size stops falling fast and the artefacts start.
	jpegQuality = 85
)

// Avatar decodes an uploaded image, centre-crops it to a square, scales it to
// avatarSize and re-encodes it as JPEG.
//
// **The re-encode is the point, and not for size.** An upload is
// attacker-controlled bytes; round-tripping them through a decoder is what makes
// the stored file provably an image, and it strips EXIF as a side effect —
// which matters because phone photos carry GPS and a profile photo is public.
//
// DecodeConfig runs first: it reads only the header, so the dimensions are known
// before Decode allocates a pixel. A request-body limit cannot catch a decode
// bomb — a 60000-square PNG is forty bytes of header — and this can.
func Avatar(r io.Reader) ([]byte, error) {
	// Buffered because the reader is consumed twice: once for the header, once
	// for the real decode.
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("images: read: %w", err)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrNotAnImage
	}
	if err := validate.ImageDimensions(cfg.Width, cfg.Height); err != nil {
		return nil, err
	}

	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrNotAnImage
	}

	dst := image.NewRGBA(image.Rect(0, 0, avatarSize, avatarSize))
	// CatmullRom rather than NearestNeighbor: this is a photograph shrunk by a
	// large factor, where nearest-neighbour aliases badly.
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, coverRect(src.Bounds()), draw.Over, nil)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("images: encode: %w", err)
	}
	return out.Bytes(), nil
}

// coverRect is the largest centred square inside b — what object-fit: cover
// does, expressed as a source rectangle.
//
// Scaling the full rectangle into a square instead would squash every photo
// that is not already square, and would still produce a valid 256x256 JPEG,
// which is why a test compares a tall source against a wide one rather than
// checking the output's size.
func coverRect(b image.Rectangle) image.Rectangle {
	w, h := b.Dx(), b.Dy()

	side := min(w, h)
	x := b.Min.X + (w-side)/2
	y := b.Min.Y + (h-side)/2

	return image.Rect(x, y, x+side, y+side)
}

// Hash is a short content fingerprint, used to build a cache-busting URL.
// Twelve hex characters is plenty to tell one person's avatars apart; this is
// not an integrity check.
func Hash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:12]
}
