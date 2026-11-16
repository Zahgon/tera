package tera

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanLoadFromGlob(t *testing.T) {
	for _, pattern := range []string{"examples/basic/templates/**/*", "examples/basic/templates/**/*.{html,xml}"} {
		files, err := LoadGlob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 3 {
			t.Fatalf("%s: expected 3 templates, got %v", pattern, files)
		}
		names := map[string]bool{}
		for _, f := range files {
			names[f.Name] = true
		}
		if !names["base.html"] || !names["users/profile.html"] {
			t.Fatalf("%s: templates %v, want base.html and users/profile.html among them", pattern, names)
		}
	}
}
func TestGlobWorkWithAbsolutePaths(t *testing.T) {
	for _, suffix := range []string{"/*.html", "/**/*.html"} {
		dir := t.TempDir()
		writeFixture(t, filepath.Join(dir, "hey.html"), "")
		writeFixture(t, filepath.Join(dir, "ho.html"), "")
		files, err := LoadGlob(filepath.ToSlash(dir) + suffix)
		if err != nil || len(files) != 2 {
			t.Fatalf("expected 2 files, got %v, %v", files, err)
		}
	}
}
func TestGlobWorkWithPathsStartingWithDots(t *testing.T) {
	dir, err := os.MkdirTemp(".", "tera_test_scratchspace")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	writeFixture(t, filepath.Join(dir, "hey.html"), "")
	writeFixture(t, filepath.Join(dir, "ho.html"), "")
	files, err := LoadGlob("./" + filepath.ToSlash(dir) + "/**/*.html")
	if err != nil || len(files) != 2 {
		t.Fatalf("expected 2 files, got %v, %v", files, err)
	}
}
func TestEmptyListOnInvalidGlob(t *testing.T) {
	files, err := LoadGlob(`\dev/null/*`)
	if err != nil || len(files) != 0 {
		t.Fatalf("expected empty list, got %v, %v", files, err)
	}
}
func TestGlobWithoutDirectoryResolvesAgainstCWD(t *testing.T) {
	// This fixture is Go source, so the test survives removal of Cargo manifests.
	files, err := LoadGlob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Name == "globbing_test.go" {
			return
		}
	}
	t.Fatal("current-directory glob did not find globbing_test.go")
}
func TestGlobWithWildcardInDirectoryComponent(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "templates"), 0700); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(dir, "templates", "hey.html"), "")
	files, err := LoadGlob(filepath.ToSlash(dir) + "/templ*/*.html")
	if err != nil || len(files) != 1 || files[0].Name != "templates/hey.html" {
		t.Fatalf("expected templates/hey.html, got %v, %v", files, err)
	}
}
