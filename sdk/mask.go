package sdk

import (
	"regexp"
	"strings"
)

// Mask replaces whole-word mentions of a subject's own name in prose with a placeholder, so a
// classifier judges the text and not the name: asked whether a doc says a function reports a key
// as present, a classifier believes a function named Missing over the doc that describes it.
// Names shorter than two characters are left alone; matching is case-sensitive.
func Mask(text, name, as string) string {
	if len(name) < 2 || as == "" {
		return text
	}
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	return re.ReplaceAllLiteralString(text, as)
}

// MaskAll applies Mask for every name, longest first, so a name that contains another is replaced
// whole.
func MaskAll(text string, names map[string]string) string {
	keys := make([]string, 0, len(names))
	for k := range names {
		keys = append(keys, k)
	}
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if len(keys[j]) > len(keys[i]) || (len(keys[j]) == len(keys[i]) && strings.Compare(keys[j], keys[i]) < 0) {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, k := range keys {
		text = Mask(text, k, names[k])
	}
	return text
}
