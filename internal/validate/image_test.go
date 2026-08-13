package validate

import (
	"errors"
	"testing"
)

func TestImageDimensions(t *testing.T) {
	t.Run("an ordinary photo is allowed", func(t *testing.T) {
		if err := ImageDimensions(4032, 3024); err != nil {
			t.Errorf("a 12MP phone photo was rejected: %v", err)
		}
	})

	t.Run("exactly at the limit is allowed", func(t *testing.T) {
		// 20,000,000 pixels exactly.
		if err := ImageDimensions(5000, 4000); err != nil {
			t.Errorf("the limit itself was rejected: %v", err)
		}
	})

	// The decode bomb, expressed as numbers rather than as a file. A 50000
	// square PNG compresses to a few kilobytes and allocates ten gigabytes.
	t.Run("a decode bomb is rejected", func(t *testing.T) {
		if err := ImageDimensions(50000, 50000); !errors.Is(err, ErrRange) {
			t.Errorf("err = %v, want ErrRange", err)
		}
	})

	t.Run("a zero or negative dimension is rejected", func(t *testing.T) {
		for _, d := range [][2]int{{0, 100}, {100, 0}, {-1, 100}} {
			if err := ImageDimensions(d[0], d[1]); !errors.Is(err, ErrRange) {
				t.Errorf("ImageDimensions(%d, %d) = %v, want ErrRange", d[0], d[1], err)
			}
		}
	})

	// The implementation divides rather than multiplying, and this is the input
	// that tells the two apart. (1<<62)*(1<<62) is 2^124, which wraps to
	// exactly 0 in a 64-bit int — so a multiplying version compares 0 against
	// the limit and *accepts* the image. 1<<30 squared is 2^60, which fits,
	// which is why an earlier version of this test passed against the bug.
	t.Run("does not overflow on absurd input", func(t *testing.T) {
		if err := ImageDimensions(1<<62, 1<<62); !errors.Is(err, ErrRange) {
			t.Errorf("err = %v, want ErrRange", err)
		}
	})
}
