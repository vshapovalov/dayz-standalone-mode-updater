package steam

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestParseDetails(t *testing.T) {
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(`{
"response":{"publishedfiledetails":[{"publishedfileid":"1","title":"CF","time_updated":1700000000}]}
}`))}
	mods, err := parseDetails(resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 1 || mods[0].ID != "1" {
		t.Fatalf("unexpected parsed result: %#v", mods)
	}
}
