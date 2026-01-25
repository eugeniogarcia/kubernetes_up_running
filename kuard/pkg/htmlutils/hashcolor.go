package htmlutils

import (
	"fmt"
	"hash/fnv"
)

// ColorFromString returns a CSS color string by hashing the incoming string.
// Used to visually identify different versions.
func ColorFromString(s string) string {
	h := fnv.New32a()
	h.Write([]byte(s))
	hue := h.Sum32() % 360
	return fmt.Sprintf("hsl(%d,100%%,50%%)", hue)
}
