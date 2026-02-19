package sftpsync

import (
	"testing"
	"time"
)

func TestBuildPlanDetectsChangesAndConflicts(t *testing.T) {
	local := map[string]treeEntry{
		"dir":          {Path: "dir", IsDir: true},
		"dir/file.txt": {Path: "dir/file.txt", IsDir: false, Size: 10, MTime: 100},
		"new.txt":      {Path: "new.txt", IsDir: false, Size: 1, MTime: 50},
	}
	remote := map[string]treeEntry{
		"dir":          {Path: "dir", IsDir: false, Size: 3, MTime: 1}, // type conflict
		"dir/file.txt": {Path: "dir/file.txt", IsDir: false, Size: 9, MTime: 100},
		"extra.txt":    {Path: "extra.txt", IsDir: false},
	}
	plan := buildPlan(local, remote)
	if len(plan.deleteTypeConflicts) != 1 || plan.deleteTypeConflicts[0].Path != "dir" {
		t.Fatalf("unexpected type conflicts: %#v", plan.deleteTypeConflicts)
	}
	if len(plan.mkdirs) != 1 || plan.mkdirs[0].Path != "dir" {
		t.Fatalf("unexpected mkdirs: %#v", plan.mkdirs)
	}
	if len(plan.uploads) != 2 {
		t.Fatalf("expected 2 uploads, got %#v", plan.uploads)
	}
	if len(plan.deleteExtrasFiles) != 1 || plan.deleteExtrasFiles[0].Path != "extra.txt" {
		t.Fatalf("unexpected extra files: %#v", plan.deleteExtrasFiles)
	}
}

func TestDeletionOrderingDirectoriesByDescendingDepth(t *testing.T) {
	local := map[string]treeEntry{}
	remote := map[string]treeEntry{
		"a":     {Path: "a", IsDir: true},
		"a/b":   {Path: "a/b", IsDir: true},
		"a/b/c": {Path: "a/b/c", IsDir: true},
	}
	plan := buildPlan(local, remote)
	got := []string{plan.deleteExtrasDirs[0].Path, plan.deleteExtrasDirs[1].Path, plan.deleteExtrasDirs[2].Path}
	want := []string{"a/b/c", "a/b", "a"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dir ordering mismatch got=%v want=%v", got, want)
		}
	}
}

func TestFileEqualityUsesMtimeSeconds(t *testing.T) {
	base := time.Unix(100, 0).UTC()
	local := map[string]treeEntry{
		"file.bin": {Path: "file.bin", IsDir: false, Size: 5, MTime: base.Unix(), ModTime: base},
	}
	remote := map[string]treeEntry{
		"file.bin": {Path: "file.bin", IsDir: false, Size: 5, MTime: base.Add(900 * time.Millisecond).Truncate(time.Second).Unix()},
	}
	plan := buildPlan(local, remote)
	if len(plan.uploads) != 0 {
		t.Fatalf("expected no uploads when size+mtime seconds match, got %#v", plan.uploads)
	}
}
