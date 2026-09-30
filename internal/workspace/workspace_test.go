package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thebrazenbeard/workbridge/internal/config"
)

func testConfig(root string, writable bool) *config.Config {
	cfg := &config.Config{
		Schema:    config.Schema,
		ReadRoots: []string{root},
		Limits:    config.Limits{MaxReadBytes: 1024, MaxWriteBytes: 1024, MaxDirectoryEntries: 10},
		Process:   config.ProcessConfig{MaxRuntimeSeconds: 1, MaxOutputBytes: 1024, MaxArgs: 8},
		HTTP:      config.HTTPConfig{Listen: "127.0.0.1:8765", Path: "/mcp"},
	}
	if writable {
		cfg.WriteRoots = []string{root}
	}
	return cfg
}

func TestReadListStatAndWriteBoundaries(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "hello.txt")
	if err := os.WriteFile(file, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := New(testConfig(root, true))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	text, err := s.ReadText(file)
	if err != nil || text != "hello" {
		t.Fatalf("read=%q err=%v", text, err)
	}
	entries, err := s.List(root)
	if err != nil || len(entries) != 1 || entries[0].Name != "hello.txt" {
		t.Fatalf("list=%v err=%v", entries, err)
	}
	if entries[0].Path != file || entries[0].Type != "file" || entries[0].IsSymlink || entries[0].SizeBytes != 5 {
		t.Fatalf("unexpected normalized entry metadata: %#v", entries[0])
	}
	stat, err := s.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Name != "hello.txt" || stat.Path != file || stat.Type != "file" || stat.IsSymlink || stat.SizeBytes != 5 || stat.MtimeNS == 0 {
		t.Fatalf("unexpected normalized stat metadata: %#v", stat)
	}
	newFile := filepath.Join(root, "new.txt")
	if _, err := s.WriteText(newFile, "new", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteText(newFile, "changed", false); err == nil {
		t.Fatal("overwrite occurred without overwrite=true")
	}
	if _, err := s.WriteText(newFile, "changed", true); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(newFile)
	if string(got) != "changed" {
		t.Fatalf("unexpected write: %q", got)
	}
	nestedDir := filepath.Join(root, "nested", "deep")
	if err := s.Mkdir(nestedDir, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Mkdir(nestedDir, false); err != nil {
		t.Fatalf("idempotent mkdir failed: %v", err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(newFile, link); err == nil {
		if _, err := s.WriteText(link, "through-link", true); err == nil {
			t.Fatal("symbolic-link overwrite accepted")
		}
	}
}

func TestWriteDisabledAndOutsideRootDenied(t *testing.T) {
	root := t.TempDir()
	s, err := New(testConfig(root, false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.WriteText(filepath.Join(root, "x.txt"), "x", false); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled write accepted: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "x.txt")
	if _, err := s.ReadText(outside); err == nil {
		t.Fatal("outside read accepted")
	}
}

func TestStatReportsSymlinkWithoutBreakingCompatibilityFields(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(filepath.Base(target), link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	s, err := New(testConfig(root, false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	stat, err := s.Stat(link)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Name != "link.txt" || stat.Type != "symlink" || !stat.IsSymlink {
		t.Fatalf("symlink metadata lost: %#v", stat)
	}
	if stat.IsDir || stat.Size != int64(len("target")) {
		t.Fatalf("legacy compatibility fields did not follow symlink target: %#v", stat)
	}
}

func TestMoveIsSameRootAndNoReplace(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.mkv")
	destination := filepath.Join(root, "renamed.mkv")
	if err := os.WriteFile(source, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := New(testConfig(root, true))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Move(source, destination); err != nil {
		t.Fatalf("move failed: %v", err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source still exists after move: %v", err)
	}
	if got, err := os.ReadFile(destination); err != nil || string(got) != "media" {
		t.Fatalf("destination mismatch: %q err=%v", got, err)
	}
	second := filepath.Join(root, "second.mkv")
	if err := os.WriteFile(second, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Move(second, destination); err == nil || !strings.Contains(err.Error(), "destination already exists") {
		t.Fatalf("existing destination was overwritten or wrong error: %v", err)
	}
}

func TestMoveOutsideWriteRootDenied(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.mkv")
	if err := os.WriteFile(source, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := New(testConfig(root, true))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	outside := filepath.Join(t.TempDir(), "outside.mkv")
	if err := s.Move(source, outside); err == nil {
		t.Fatal("move outside configured write root succeeded")
	}
}

func TestMoveRejectsDirectoryWithoutModifyingContents(t *testing.T) {
	root := t.TempDir()
	from := filepath.Join(root, "folder")
	if err := os.Mkdir(from, 0o700); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(from, "keep.txt")
	if err := os.WriteFile(original, []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(testConfig(root, true))
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	to := filepath.Join(root, "renamed-folder")
	if err := svc.Move(from, to); err == nil || !strings.Contains(err.Error(), "regular files only") {
		t.Fatalf("directory move must fail closed: %v", err)
	}
	if b, err := os.ReadFile(original); err != nil || string(b) != "untouched" {
		t.Fatalf("directory content changed: %q %v", b, err)
	}
}
