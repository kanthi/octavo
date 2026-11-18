package library

import (
	"fmt"
	"math"
	"testing"
)

func TestColoursAreDistinct(t *testing.T) {
	// The smallest difference between any two cards, by number of books.
	for n, want := range map[int]float64{2: 0.12, 5: 0.08, 8: 0.06, 14: 0.05, 18: 0.04, 24: 0.03, 100: 0.005} {
		var names []string
		for i := range n {
			names = append(names, fmt.Sprintf("Book%d", i))
		}
		got := colours(names)
		closest := math.Inf(1)
		for i, a := range names {
			for _, b := range names[i+1:] {
				closest = math.Min(closest, got[a].distance(got[b]))
			}
		}
		if closest < want {
			t.Errorf("%d books: the two closest colours are %.3f apart, want at least %.3f", n, closest, want)
		}
		// Up to 14 books, no two share a hue.
		for i, a := range names {
			for _, b := range names[i+1:] {
				d := math.Abs(float64(got[a].Hue - got[b].Hue))
				if d = math.Min(d, 360-d); n <= 14 && d < 20 {
					t.Errorf("%d books: %s and %s share a hue (%d° and %d°)", n, a, b, got[a].Hue, got[b].Hue)
				}
			}
		}
	}
	// A book keeps its colour when another book is added.
	before := colours([]string{"Go", "Linux"})
	after := colours([]string{"Go", "Linux", "Rust"})
	if before["Go"] != after["Go"] || before["Linux"] != after["Linux"] {
		t.Errorf("adding a book changed other books' colours: %v -> %v", before, after)
	}
}
