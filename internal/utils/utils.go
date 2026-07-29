package utils

import (
	"maps"
	"strings"

	"github.com/fatih/structs"
	"tideland.dev/go/slices"
)

// FieldTagNames returns a slice of the tag names for a []*structs.Field and the given tag
func FieldTagNames(fields []*structs.Field, tag string) (names []string) {
	for _, f := range fields {
		if f == nil {
			continue
		}
		t := f.Tag(tag)
		if i := strings.IndexRune(t, ','); i > 0 {
			t = t[:i]
		}
		if t != "" && t != "-" {
			names = append(names, t)
		}
	}
	return
}

// MergeMaps merges two or more maps into on; overwrite determines if values are overwritten if already set or not
func MergeMaps(overwrite bool, mm ...map[string]any) map[string]any {
	if !overwrite {
		return MergeMaps(true, slices.Reverse(mm)...)
	}
	all := make(map[string]any)
	for _, m := range mm {
		maps.Copy(all, m)
	}
	return all
}
