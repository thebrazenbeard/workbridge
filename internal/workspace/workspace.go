package workspace

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf8"

	"github.com/thebrazenbeard/workbridge/internal/config"
	"github.com/thebrazenbeard/workbridge/internal/policy"
)

type Service struct {
	read       *policy.RootPolicy
	write      *policy.RootPolicy
	maxRead    int64
	maxWrite   int64
	maxEntries int
}

type Entry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Type      string `json:"type"`
	IsSymlink bool   `json:"is_symlink"`
	SizeBytes int64  `json:"size_bytes"`
	MtimeNS   int64  `json:"mtime_ns"`

	// Compatibility fields retained for existing WorkBridge clients.
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

type Stat struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Type      string `json:"type"`
	IsSymlink bool   `json:"is_symlink"`
	SizeBytes int64  `json:"size_bytes"`
	MtimeNS   int64  `json:"mtime_ns"`

	// Compatibility fields retain the prior follow-final-link view.
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size"`
	Mode     string `json:"mode"`
	Modified string `json:"modified"`
}

func New(cfg *config.Config) (*Service, error) {
	read, err := policy.NewRootPolicy(cfg.ReadRoots)
	if err != nil {
		return nil, err
	}
	write, err := policy.NewRootPolicy(cfg.WriteRoots)
	if err != nil {
		read.Close()
		return nil, err
	}
	return &Service{
		read: read, write: write,
		maxRead: cfg.Limits.MaxReadBytes,
		maxWrite: cfg.Limits.MaxWriteBytes,
		maxEntries: cfg.Limits.MaxDirectoryEntries,
	}, nil
}

func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	var first error
	if s.read != nil {
		first = s.read.Close()
	}
	if s.write != nil {
		if err := s.write.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (s *Service) CanWrite() bool { return !s.write.Empty() }

func (s *Service) List(path string) ([]Entry, error) {
	f, resolvedDir, err := s.read.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("path is not a directory")
	}
	items, err := f.ReadDir(s.maxEntries + 1)
	if err != nil {
		return nil, err
	}
	if len(items) > s.maxEntries {
		return nil, fmt.Errorf("directory exceeds entry limit %d", s.maxEntries)
	}
	out := make([]Entry, 0, len(items))
	for _, item := range items {
		itemInfo, err := item.Info()
		if err != nil {
			return nil, err
		}
		childPath := filepath.Join(resolvedDir, item.Name())
		lstat, _, err := s.read.Lstat(childPath)
		if err != nil {
			return nil, err
		}
		out = append(out, Entry{
			Name: item.Name(),
			Path: childPath,
			Type: classifyMode(lstat.Mode()),
			IsSymlink: lstat.Mode()&os.ModeSymlink != 0,
			SizeBytes: lstat.Size(),
			MtimeNS: lstat.ModTime().UnixNano(),
			IsDir: item.IsDir(),
			Size: itemInfo.Size(),
			Modified: itemInfo.ModTime().UTC().Format("2006-01-02T15:04:05.999999999Z"),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Service) Stat(path string) (*Stat, error) {
	leaf, resolved, err := s.read.Lstat(path)
	if err != nil {
		return nil, err
	}
	followed, _, err := s.read.Stat(path)
	if err != nil {
		return nil, err
	}
	return &Stat{
		Name: filepath.Base(resolved),
		Path: resolved,
		Type: classifyMode(leaf.Mode()),
		IsSymlink: leaf.Mode()&os.ModeSymlink != 0,
		SizeBytes: leaf.Size(),
		MtimeNS: leaf.ModTime().UnixNano(),
		IsDir: followed.IsDir(),
		Size: followed.Size(),
		Mode: followed.Mode().String(),
		Modified: followed.ModTime().UTC().Format("2006-01-02T15:04:05.999999999Z"),
	}, nil
}

func classifyMode(mode os.FileMode) string {
	switch {
	case mode&os.ModeSymlink != 0:
		return "symlink"
	case mode.IsDir():
		return "directory"
	case mode.IsRegular():
		return "file"
	default:
		return "other"
	}
}

func (s *Service) ReadText(path string) (string, error) {
	f, _, err := s.read.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("path is not a regular file")
	}
	if info.Size() > s.maxRead {
		return "", fmt.Errorf("file size %d exceeds read limit %d", info.Size(), s.maxRead)
	}
	data, err := io.ReadAll(io.LimitReader(f, s.maxRead+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > s.maxRead {
		return "", fmt.Errorf("read exceeds limit %d", s.maxRead)
	}
	if !utf8.Valid(data) {
		return "", errors.New("file is not valid UTF-8 text")
	}
	return string(data), nil
}

func (s *Service) WriteText(path, content string, overwrite bool) (int, error) {
	if s.write.Empty() {
		return 0, errors.New("write capability is disabled")
	}
	if int64(len(content)) > s.maxWrite {
		return 0, fmt.Errorf("write size %d exceeds limit %d", len(content), s.maxWrite)
	}
	if info, _, err := s.write.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return 0, errors.New("refusing to overwrite a symbolic link")
		}
		if info.IsDir() {
			return 0, errors.New("refusing to overwrite a directory")
		}
		if !overwrite {
			return 0, errors.New("target already exists; set overwrite=true to replace it")
		}
	} else if !os.IsNotExist(err) {
		return 0, err
	}

	if _, err := s.write.AtomicWrite(path, []byte(content), 0o600, overwrite); err != nil {
		return 0, err
	}
	return len(content), nil
}

func (s *Service) Move(source, destination string) error {
	if s.write.Empty() {
		return errors.New("write capability is disabled")
	}
	_, _, err := s.write.RenameNoReplace(source, destination)
	return err
}

func (s *Service) Mkdir(path string, parents bool) error {
	if s.write.Empty() {
		return errors.New("write capability is disabled")
	}
	_, err := s.write.Mkdir(path, 0o700, parents)
	return err
}
