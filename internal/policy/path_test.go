package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootPolicyAllowsInsideAndRejectsOutside(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "a.txt")
	if err := os.WriteFile(inside, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := NewRootPolicy([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err := p.ResolveExisting(inside); err != nil {
		t.Fatalf("inside denied: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "no.txt")
	if err := os.WriteFile(outside, []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ResolveExisting(outside); err == nil {
		t.Fatal("outside path accepted")
	}
}

func TestRootPolicyRejectsSymlinkEscapeAtOperationTime(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	p, err := NewRootPolicy([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, _, err := p.OpenFile(
		filepath.Join(link, "x.txt"),
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0o600,
	); err == nil {
		t.Fatal("os.Root permitted symlink escape")
	}
}

func TestRootPolicyRejectsParentTraversal(t *testing.T) {
	root := t.TempDir()
	p, err := NewRootPolicy([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, _, err := p.OpenFile(
		filepath.Join(root, "..", "escape.txt"),
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0o600,
	); err == nil {
		t.Fatal("parent traversal accepted")
	}
}


func TestRenameNoReplaceRefusesExistingDestination(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "a.txt")
	destination := filepath.Join(root, "b.txt")
	if err := os.WriteFile(source, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := NewRootPolicy([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, _, err := p.RenameNoReplace(source, destination); err == nil {
		t.Fatal("existing destination was replaced")
	}
	got, err := os.ReadFile(destination)
	if err != nil || string(got) != "b" {
		t.Fatalf("destination changed: %q err=%v", got, err)
	}
}
