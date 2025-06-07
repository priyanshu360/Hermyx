package regex

import (
	"regexp"
	"strings"
)

func CombinePattenrs(patterns []string) *regexp.Regexp {
	combined := "(?:" + strings.Join(patterns, ")|(?:") + ")"
	return regexp.MustCompile(combined)
}
