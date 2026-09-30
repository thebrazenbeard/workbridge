package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

func TestRenameNoReplaceConcurrentClaimsNeverClobber(t *testing.T) {
	root := t.TempDir()
	p, err := NewRootPolicy([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	const workers = 32
	target := filepath.Join(root, "shared-target.txt")
	starts := make(chan struct{})
	var wg sync.WaitGroup
	wins := make(chan int, workers)
	for i := 0; i < workers; i++ {
		path := filepath.Join(root, fmt.Sprintf("source-%02d.txt", i))
		if err := os.WriteFile(path, []byte(fmt.Sprintf("writer-%02d", i)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-starts
			path := filepath.Join(root, fmt.Sprintf("source-%02d.txt", i))
			if _, _, err := p.RenameNoReplace(path, target); err == nil {
				wins <- i
			}
		}(i)
	}
	close(starts)
	wg.Wait()
	close(wins)
	count := 0
	winner := -1
	for i := range wins {
		count++
		winner = i
	}
	if count != 1 {
		t.Fatalf("expected exactly one success, got %d", count)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != fmt.Sprintf("writer-%02d", winner) {
		t.Fatalf("destination corrupted: %q %v", got, err)
	}
	for i := 0; i < workers; i++ {
		source := filepath.Join(root, fmt.Sprintf("source-%02d.txt", i))
		b, err := os.ReadFile(source)
		if i == winner {
			if !os.IsNotExist(err) {
				t.Fatalf("winner source remains: %v", err)
			}
		} else if err != nil || string(b) != fmt.Sprintf("writer-%02d", i) {
			t.Fatalf("loser source %d altered: %q %v", i, b, err)
		}
	}
}

func TestRenameNoReplaceRejectsNonregularSource(t *testing.T) {
	root := t.TempDir()
	p, err := NewRootPolicy([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	directory := filepath.Join(root, "source-dir")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "target-dir")
	if _, _, err := p.RenameNoReplace(directory, target); err == nil {
		t.Fatal("directory rename unexpectedly permitted")
	}
	if _, err := os.Stat(directory); err != nil {
		t.Fatalf("directory altered: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target created: %v", err)
	}
}
