package relayversion

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse parses "1.2.3" or "v1.2.3" into [major,minor,patch].
func Parse(v string) ([3]int, error) {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	parts := strings.SplitN(v, "-", 2)
	nums := strings.Split(parts[0], ".")
	if len(nums) < 1 || len(nums) > 3 {
		return [3]int{}, fmt.Errorf("invalid version: %q", v)
	}
	var out [3]int
	for i, n := range nums {
		x, err := strconv.Atoi(n)
		if err != nil || x < 0 {
			return [3]int{}, fmt.Errorf("invalid version: %q", v)
		}
		out[i] = x
	}
	return out, nil
}

// AtLeast reports whether v >= min.
func AtLeast(v, min string) (bool, error) {
	vv, err := Parse(v)
	if err != nil {
		return false, err
	}
	mm, err := Parse(min)
	if err != nil {
		return false, err
	}
	for i := 0; i < 3; i++ {
		if vv[i] > mm[i] {
			return true, nil
		}
		if vv[i] < mm[i] {
			return false, nil
		}
	}
	return true, nil
}
