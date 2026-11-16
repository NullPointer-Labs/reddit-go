package compare

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sort"
	"sync/atomic"
)

type Stats struct {
	ReadsCompared   atomic.Int64 `json:"-"`
	ReadMismatches  atomic.Int64 `json:"-"`
	ReadRaces       atomic.Int64 `json:"-"`
	WritesCompared  atomic.Int64 `json:"-"`
	WriteMismatches atomic.Int64 `json:"-"`
}

func (s *Stats) Snapshot() map[string]int64 {
	return map[string]int64{
		"reads_compared":   s.ReadsCompared.Load(),
		"read_mismatches":  s.ReadMismatches.Load(),
		"read_races":       s.ReadRaces.Load(),
		"writes_compared":  s.WritesCompared.Load(),
		"write_mismatches": s.WriteMismatches.Load(),
	}
}

func Diff(legacy, mine []byte, ignore map[string]bool) []string {
	var a, b any
	if err := json.Unmarshal(legacy, &a); err != nil {
		return []string{fmt.Sprintf("legacy body is not JSON: %v", err)}
	}
	if err := json.Unmarshal(mine, &b); err != nil {
		return []string{fmt.Sprintf("go body is not JSON: %v", err)}
	}
	var out []string
	walk("", a, b, ignore, &out)
	return out
}

func walk(path string, a, b any, ignore map[string]bool, out *[]string) {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			*out = append(*out, fmt.Sprintf("%s: legacy=%v go=%v", path, a, b))
			return
		}
		keys := map[string]bool{}
		for k := range av {
			keys[k] = true
		}
		for k := range bv {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			if ignore[k] {
				continue
			}
			x, inA := av[k]
			y, inB := bv[k]
			p := join(path, k)
			switch {
			case !inA:
				*out = append(*out, fmt.Sprintf("%s: missing in legacy, go=%v", p, y))
			case !inB:
				*out = append(*out, fmt.Sprintf("%s: missing in go, legacy=%v", p, x))
			default:
				walk(p, x, y, ignore, out)
			}
		}
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			*out = append(*out, fmt.Sprintf("%s: legacy has %d items, go has %v", path, len(av), lenOf(b)))
			return
		}
		for i := range av {
			walk(fmt.Sprintf("%s[%d]", path, i), av[i], bv[i], ignore, out)
		}
	default:
		if !reflect.DeepEqual(a, b) {
			*out = append(*out, fmt.Sprintf("%s: legacy=%v go=%v", path, a, b))
		}
	}
}

func join(path, k string) string {
	if path == "" {
		return k
	}
	return path + "." + k
}

func lenOf(v any) any {
	if s, ok := v.([]any); ok {
		return len(s)
	}
	return v
}

func LogMismatch(kind, route string, diffs []string) {
	if len(diffs) > 3 {
		diffs = append(diffs[:3], fmt.Sprintf("... +%d more", len(diffs)-3))
	}
	log.Printf("MISMATCH %s %s %v", kind, route, diffs)
}
