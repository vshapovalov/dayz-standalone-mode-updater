package util

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"CF Tools":            "cf-tools",
		"  [DZ] Super_Mod!!!": "dz-super-mod",
		"###":                 "mod",
	}
	for in, expected := range cases {
		if got := Slugify(in); got != expected {
			t.Fatalf("slugify(%q)=%q want %q", in, got, expected)
		}
	}
}
