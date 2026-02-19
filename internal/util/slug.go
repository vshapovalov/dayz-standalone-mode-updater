package util

import (
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func Slugify(in string) string {
	s := strings.ToLower(strings.TrimSpace(in))
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "mod"
	}
	return s
}
