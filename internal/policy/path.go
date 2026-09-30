package policy

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type rootEntry struct {
	paths []string
	root  *os.Root
}

type RootPolicy struct {
	roots []rootEntry
}

func NewRootPolicy(roots []string) (*RootPolicy, error) {
	p := &RootPolicy{}
	for _, raw := range roots {
		if !filepath.IsAbs(raw) {
			p.Close()
			return nil, fmt.Errorf("root must be absolute: %q", raw)
		}
		configured, err := filepath.Abs(filepath.Clean(raw))
		if err != nil {
			p.Close()
			return nil, err
		}
		resolved, err := filepath.EvalSymlinks(configured)
		if err != nil {
			p.Close()
			return nil, fmt.Errorf("resolve root %q: %w", raw, err)
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			p.Close()
			return nil, err
		}
		resolved = filepath.Clean(resolved)
		info, err := os.Stat(resolved)
		if err != nil {
			p.Close()
			return nil, fmt.Errorf("stat root %q: %w", resolved, err)
		}
		if !info.IsDir() {
			p.Close()
			return nil, fmt.Errorf("root is not a directory: %q", resolved)
		}
		r, err := os.OpenRoot(resolved)
		if err != nil {
			p.Close()
			return nil, fmt.Errorf("open root %q: %w", resolved, err)
		}
		aliases := []string{configured}
		if resolved != configured {
			aliases = append(aliases, resolved)
		}
		p.roots = append(p.roots, rootEntry{paths: aliases, root: r})
	}
	sort.Slice(p.roots, func(i, j int) bool {
		return longestPath(p.roots[i].paths) > longestPath(p.roots[j].paths)
	})
	return p, nil
}

func longestPath(paths []string) int {
	longest := 0
	for _, path := range paths {
		if len(path) > longest {
			longest = len(path)
		}
	}
	return longest
}

func (p *RootPolicy) Close() error {
	if p == nil {
		return nil
	}
	var first error
	for i := range p.roots {
		if p.roots[i].root == nil {
			continue
		}
		if err := p.roots[i].root.Close(); err != nil && first == nil {
			first = err
		}
		p.roots[i].root = nil
	}
	return first
}

func (p *RootPolicy) Empty() bool { return p == nil || len(p.roots) == 0 }

func (p *RootPolicy) ResolveExisting(path string) (string, error) {
	entry, rel, abs, err := p.match(path)
	if err != nil {
		return "", err
	}
	if _, err := entry.root.Stat(rel); err != nil {
		return "", err
	}
	return abs, nil
}

func (p *RootPolicy) Open(path string) (*os.File, string, error) {
	entry, rel, abs, err := p.match(path)
	if err != nil {
		return nil, "", err
	}
	f, err := entry.root.Open(rel)
	if err != nil {
		return nil, "", err
	}
	return f, abs, nil
}

func (p *RootPolicy) OpenFile(path string, flag int, perm fs.FileMode) (*os.File, string, error) {
	entry, rel, abs, err := p.match(path)
	if err != nil {
		return nil, "", err
	}
	f, err := entry.root.OpenFile(rel, flag, perm)
	if err != nil {
		return nil, "", err
	}
	return f, abs, nil
}

func (p *RootPolicy) Stat(path string) (fs.FileInfo, string, error) {
	entry, rel, abs, err := p.match(path)
	if err != nil {
		return nil, "", err
	}
	info, err := entry.root.Stat(rel)
	if err != nil {
		return nil, "", err
	}
	return info, abs, nil
}

func (p *RootPolicy) Lstat(path string) (fs.FileInfo, string, error) {
	entry, rel, abs, err := p.match(path)
	if err != nil {
		return nil, "", err
	}
	info, err := entry.root.Lstat(rel)
	if err != nil {
		return nil, "", err
	}
	return info, abs, nil
}

func (p *RootPolicy) RenameNoReplace(source, destination string) (string, string, error) {
	sourceEntry, sourceRel, sourceAbs, err := p.match(source)
	if err != nil {
		return "", "", err
	}
	destinationEntry, destinationRel, destinationAbs, err := p.match(destination)
	if err != nil {
		return "", "", err
	}
	if sourceEntry != destinationEntry {
		return "", "", errors.New("source and destination must be within the same configured root")
	}
	// Rename after an Lstat(destination) check is NOT no-replace: another
	// process can create the destination between the check and Rename, at
	// which point standard rename semantics can overwrite that file.
	//
	// Root.Link atomically creates a directory entry only when the target
	// does not already exist. Remove the source only after that succeeds.
	// This supports regular files; directories and symlinks are deliberately
	// unsupported rather than risking clobber or following a link.
	sourceInfo, err := sourceEntry.root.Lstat(sourceRel)
	if err != nil {
		return "", "", err
	}
	if !sourceInfo.Mode().IsRegular() {
		return "", "", errors.New("workspace_move supports regular files only")
	}
	if err := sourceEntry.root.Link(sourceRel, destinationRel); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", "", errors.New("destination already exists")
		}
		return "", "", fmt.Errorf("create no-replace hard link: %w", err)
	}
	// This is intentionally not marketed as a transaction: after a crash or
	// a source-removal failure both names may remain. In that situation the
	// caller must reconcile explicitly rather than blindly repeating a move.
	if err := sourceEntry.root.Remove(sourceRel); err != nil {
		return "", "", fmt.Errorf("destination created but source removal failed; inspect both paths before retrying: %w", err)
	}
	return sourceAbs, destinationAbs, nil
}

func (p *RootPolicy) Mkdir(path string, perm fs.FileMode, parents bool) (string, error) {
	entry, rel, abs, err := p.match(path)
	if err != nil {
		return "", err
	}
	if parents {
		if err := entry.root.MkdirAll(rel, perm); err != nil {
			return "", err
		}
		return abs, nil
	}
	if err := entry.root.Mkdir(rel, perm); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
		info, statErr := entry.root.Stat(rel)
		if statErr != nil {
			return "", statErr
		}
		if !info.IsDir() {
			return "", err
		}
	}
	return abs, nil
}

func (p *RootPolicy) AtomicWrite(path string, data []byte, perm fs.FileMode, overwrite bool) (string, error) {
	entry, rel, abs, err := p.match(path)
	if err != nil {
		return "", err
	}
	if !overwrite {
		f, err := entry.root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if err != nil {
			return "", err
		}
		n, writeErr := f.Write(data)
		if writeErr == nil && n != len(data) {
			writeErr = io.ErrShortWrite
		}
		if writeErr == nil {
			writeErr = f.Sync()
		}
		closeErr := f.Close()
		if writeErr != nil {
			return "", writeErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		return abs, nil
	}

	dir := filepath.Dir(rel)
	base := filepath.Base(rel)
	var tempRel string
	var f *os.File
	for attempt := 0; attempt < 8; attempt++ {
		var nonce [12]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return "", fmt.Errorf("generate atomic-write nonce: %w", err)
		}
		name := "." + base + ".workbridge-" + hex.EncodeToString(nonce[:]) + ".tmp"
		tempRel = name
		if dir != "." {
			tempRel = filepath.Join(dir, name)
		}
		f, err = entry.root.OpenFile(tempRel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		break
	}
	if f == nil {
		return "", errors.New("could not allocate atomic-write temporary file")
	}
	cleanup := func() { _ = entry.root.Remove(tempRel) }

	n, writeErr := f.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil {
		cleanup()
		return "", writeErr
	}
	if closeErr != nil {
		cleanup()
		return "", closeErr
	}
	if err := entry.root.Rename(tempRel, rel); err != nil {
		cleanup()
		return "", err
	}
	return abs, nil
}

func (p *RootPolicy) match(path string) (*rootEntry, string, string, error) {
	if p.Empty() {
		return nil, "", "", errors.New("no roots configured")
	}
	if !filepath.IsAbs(path) {
		return nil, "", "", errors.New("path must be absolute")
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, "", "", err
	}
	abs = filepath.Clean(abs)
	for i := range p.roots {
		for _, alias := range p.roots[i].paths {
			rel, err := filepath.Rel(alias, abs)
			if err != nil || !filepath.IsLocal(rel) {
				continue
			}
			return &p.roots[i], rel, abs, nil
		}
	}
	return nil, "", "", errors.New("path is outside configured roots")
}
