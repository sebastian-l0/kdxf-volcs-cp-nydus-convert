package input

import (
	"os"
	"path/filepath"
	"testing"

	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
)

func TestLoadImageLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "images.txt")
	content := "# comment\n\nrepo.invalid/ignored/nope:v0\n   # indented comment\n  vfaas-cn-beijing.cr.volces.com/swe/repo2:v2  \n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadImageLines(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].LineNumber != 3 || got[0].Raw != "repo.invalid/ignored/nope:v0" {
		t.Fatalf("first=%+v", got[0])
	}
	if got[1].LineNumber != 5 || got[1].Raw != "vfaas-cn-beijing.cr.volces.com/swe/repo2:v2" {
		t.Fatalf("second=%+v", got[1])
	}
}

func TestLoadImageLinesNotFound(t *testing.T) {
	_, err := LoadImageLines(filepath.Join(t.TempDir(), "missing.txt"))
	if apperrors.CodeOf(err) != apperrors.CodeImageFileNotFound {
		t.Fatalf("CodeOf(err)=%q err=%v", apperrors.CodeOf(err), err)
	}
}
