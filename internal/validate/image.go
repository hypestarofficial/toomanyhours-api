package validate

// maxImagePixels bounds what an upload may decode to, which the body size
// limit cannot: a 50000x50000 PNG is a few kilobytes on the wire and tens of
// gigabytes once decoded. Twenty million is comfortably above any phone camera.
const maxImagePixels = 20_000_000

// ImageDimensions checks an image's declared size before anything decodes it.
//
// Pure, and separate from the handler, because the case it guards against is
// awkward to reproduce: a decode bomb is a real file that really does exhaust
// memory. Expressed as numbers it is three lines and a test.
func ImageDimensions(width, height int) error {
	if width <= 0 || height <= 0 {
		return ErrRange
	}

	// Divide rather than multiply: width*height overflows int on a 32-bit
	// build long before the comparison would catch anything.
	if width > maxImagePixels/height {
		return ErrRange
	}

	return nil
}
