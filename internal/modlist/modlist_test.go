package modlist

import (
	"testing"

	"github.com/example/dayz-standalone-mode-updater/internal/state"
)

func TestParseHTMLModlist(t *testing.T) {
	html := `<html><body><table>
<tr data-type="ModContainer">
  <td data-type="DisplayName">   CF Tools   </td>
  <td><a data-type="Link" href="https://steamcommunity.com/sharedfiles/filedetails/?id=1564026768">Open</a></td>
</tr>
<tr data-type="ModContainer">
  <td data-type="DisplayName">Broken</td>
  <td><a data-type="Link" href="https://steamcommunity.com/sharedfiles/filedetails/?id=not-a-number">Open</a></td>
</tr>
</table></body></html>`

	warnings := 0
	mods := ParseHTMLModlist(html, func(_ string, _ ...any) {
		warnings++
	})
	if len(mods) != 1 {
		t.Fatalf("expected 1 parsed mod, got %d", len(mods))
	}
	if mods[0].DisplayName != "CF Tools" {
		t.Fatalf("unexpected display name: %q", mods[0].DisplayName)
	}
	if mods[0].WorkshopID != "1564026768" {
		t.Fatalf("unexpected workshop id: %q", mods[0].WorkshopID)
	}
	if mods[0].FolderSlug != "cf-tools" {
		t.Fatalf("unexpected folder slug: %q", mods[0].FolderSlug)
	}
	if warnings != 1 {
		t.Fatalf("expected 1 warning, got %d", warnings)
	}
}

func TestSlugifyFolder(t *testing.T) {
	cases := []struct {
		name       string
		display    string
		workshopID string
		want       string
	}{
		{name: "basic", display: "  Some Mod  Name  ", workshopID: "1", want: "some-mod-name"},
		{name: "symbols removed", display: "[DZ] Super_Mod!!!", workshopID: "2", want: "dz-supermod"},
		{name: "fallback", display: "###", workshopID: "123", want: "mod-123"},
		{name: "collapse dashes", display: "a --- b", workshopID: "3", want: "a-b"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SlugifyFolder(tc.display, tc.workshopID); got != tc.want {
				t.Fatalf("SlugifyFolder(%q,%q)=%q want %q", tc.display, tc.workshopID, got, tc.want)
			}
		})
	}
}

func TestHashModsetStableRegardlessOfOrder(t *testing.T) {
	a := HashModset([]string{"3", "1", "2"})
	b := HashModset([]string{"2", "3", "1"})
	if a != b {
		t.Fatalf("expected stable hash, got %q and %q", a, b)
	}
}

func TestApplyPollResultUpdatesState(t *testing.T) {
	st := state.State{
		Version: 1,
		Mods:    map[string]state.ModState{},
		Servers: map[string]state.ServerState{"s1": {LastModsetHash: "old", Stage: state.StageIdle}},
	}
	ApplyPollResult(&st, "s1", PollResult{
		Mods:       []ParsedMod{{DisplayName: "CF Tools", WorkshopID: "1564026768", FolderSlug: "cf-tools"}},
		SortedIDs:  []string{"1564026768"},
		ModsetHash: HashModset([]string{"1564026768"}),
	})

	srv := st.Servers["s1"]
	if !srv.NeedsModUpdate {
		t.Fatal("expected needs_mod_update=true")
	}
	if srv.Stage != state.StagePlanning {
		t.Fatalf("expected planning stage, got %q", srv.Stage)
	}
	if len(srv.LastModIDs) != 1 || srv.LastModIDs[0] != "1564026768" {
		t.Fatalf("unexpected last_mod_ids: %#v", srv.LastModIDs)
	}
	if st.Mods["1564026768"].DisplayName != "CF Tools" {
		t.Fatalf("unexpected display_name in state: %#v", st.Mods["1564026768"])
	}
}
