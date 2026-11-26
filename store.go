package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type TrashItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	OriginalVP string `json:"original_vp"`
	RealPath   string `json:"real_path"`
	IsDir      bool   `json:"is_dir"`
	Size       int64  `json:"size"`
	DeletedAt  int64  `json:"deleted_at"`
	DeletedBy  string `json:"deleted_by"`
	StoredName string `json:"stored_name"`
}

type Trash struct {
	dir   string
	file  string
	mu    sync.Mutex
	items []TrashItem
}

func newTrash(dir string) *Trash {
	t := &Trash{dir: filepath.Join(dir, ".modernfs_trash"), file: filepath.Join(dir, ".modernfs_trash.json")}
	_ = os.MkdirAll(t.dir, 0700)
	t.load()
	t.cleanOld(30 * 24 * time.Hour)
	return t
}

func (t *Trash) load() {
	b, err := os.ReadFile(t.file)
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, &t.items)
}

func (t *Trash) save() {
	b, _ := json.MarshalIndent(t.items, "", "  ")
	_ = os.WriteFile(t.file, b, 0600)
}

func (t *Trash) cleanOld(age time.Duration) {
	cutoff := time.Now().Add(-age).Unix()
	t.mu.Lock()
	defer t.mu.Unlock()
	keep := t.items[:0]
	for _, it := range t.items {
		if it.DeletedAt < cutoff {
			_ = os.RemoveAll(filepath.Join(t.dir, it.StoredName))
			continue
		}
		keep = append(keep, it)
	}
	t.items = keep
	t.save()
}

func (t *Trash) Add(real, vp, user string) error {
	info, err := os.Lstat(real)
	if err != nil {
		return err
	}
	id := randomToken(8)
	name := info.Name()
	stored := id + "_" + name
	dest := filepath.Join(t.dir, stored)
	if err := os.Rename(real, dest); err != nil {
		// 跨分区时复制再删除
		if err2 := copyTree(real, dest); err2 != nil {
			return err
		}
		_ = os.RemoveAll(real)
	}
	t.mu.Lock()
	t.items = append(t.items, TrashItem{
		ID: id, Name: name, OriginalVP: vp, RealPath: real,
		IsDir: info.IsDir(), Size: info.Size(), DeletedAt: time.Now().Unix(),
		DeletedBy: user, StoredName: stored,
	})
	sort.Slice(t.items, func(i, j int) bool { return t.items[i].DeletedAt > t.items[j].DeletedAt })
	t.save()
	t.mu.Unlock()
	return nil
}

func (t *Trash) List() []TrashItem {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]TrashItem, len(t.items))
	copy(out, t.items)
	return out
}

func (t *Trash) Find(id string) (TrashItem, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, it := range t.items {
		if it.ID == id {
			return it, true
		}
	}
	return TrashItem{}, false
}

func (t *Trash) Restore(id string) (TrashItem, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, it := range t.items {
		if it.ID != id {
			continue
		}
		src := filepath.Join(t.dir, it.StoredName)
		if _, err := os.Lstat(it.RealPath); err == nil {
			return it, errors.New("原位置已存在同名文件")
		}
		if err := os.MkdirAll(filepath.Dir(it.RealPath), 0755); err != nil {
			return it, err
		}
		if err := os.Rename(src, it.RealPath); err != nil {
			if err2 := copyTree(src, it.RealPath); err2 != nil {
				return it, err
			}
			_ = os.RemoveAll(src)
		}
		t.items = append(t.items[:i], t.items[i+1:]...)
		t.save()
		return it, nil
	}
	return TrashItem{}, errors.New("记录不存在")
}

func (t *Trash) Purge(id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, it := range t.items {
		if it.ID != id {
			continue
		}
		_ = os.RemoveAll(filepath.Join(t.dir, it.StoredName))
		t.items = append(t.items[:i], t.items[i+1:]...)
		t.save()
		return nil
	}
	return errors.New("记录不存在")
}

func (t *Trash) Empty() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, it := range t.items {
		_ = os.RemoveAll(filepath.Join(t.dir, it.StoredName))
	}
	t.items = nil
	t.save()
}

type Share struct {
	Token      string `json:"token"`
	Virtual    string `json:"virtual"`
	Real       string `json:"real"`
	Owner      string `json:"owner"`
	Name       string `json:"name"`
	IsDir      bool   `json:"is_dir"`
	CreatedAt  int64  `json:"created_at"`
	ExpiresAt  int64  `json:"expires_at"`
	Password   string `json:"password,omitempty"`
	Downloaded int64  `json:"downloads"`
}

type ShareStore struct {
	file  string
	mu    sync.Mutex
	items map[string]*Share
}

func newShareStore(dir string) *ShareStore {
	s := &ShareStore{file: filepath.Join(dir, ".modernfs_shares.json"), items: map[string]*Share{}}
	if b, err := os.ReadFile(s.file); err == nil {
		_ = json.Unmarshal(b, &s.items)
	}
	return s
}

func (s *ShareStore) save() {
	b, _ := json.MarshalIndent(s.items, "", "  ")
	_ = os.WriteFile(s.file, b, 0600)
}

func (s *ShareStore) Create(vp, real, owner, name string, isDir bool, ttl time.Duration, password string) *Share {
	sh := &Share{
		Token: randomToken(16), Virtual: vp, Real: real, Owner: owner, Name: name,
		IsDir: isDir, CreatedAt: time.Now().Unix(), Password: password,
	}
	if ttl > 0 {
		sh.ExpiresAt = time.Now().Add(ttl).Unix()
	}
	s.mu.Lock()
	s.items[sh.Token] = sh
	s.save()
	s.mu.Unlock()
	return sh
}

func (s *ShareStore) Get(token string) (*Share, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sh, ok := s.items[token]
	if !ok {
		return nil, false
	}
	if sh.ExpiresAt > 0 && sh.ExpiresAt < time.Now().Unix() {
		return nil, false
	}
	return sh, true
}

func (s *ShareStore) List(owner string) []*Share {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*Share{}
	now := time.Now().Unix()
	for _, sh := range s.items {
		if sh.Owner != owner {
			continue
		}
		if sh.ExpiresAt > 0 && sh.ExpiresAt < now {
			continue
		}
		cp := *sh
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *ShareStore) Delete(owner, token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sh, ok := s.items[token]
	if !ok || sh.Owner != owner {
		return false
	}
	delete(s.items, token)
	s.save()
	return true
}

func (s *ShareStore) BumpDownload(token string) {
	s.mu.Lock()
	if sh, ok := s.items[token]; ok {
		sh.Downloaded++
		s.save()
	}
	s.mu.Unlock()
}

func copyTree(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst, info.Mode())
	}
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}
	ents, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range ents {
		if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func safeJoin(base, name string) (string, bool) {
	clean := filepath.Clean(filepath.Join(base, name))
	if !strings.HasPrefix(clean, filepath.Clean(base)+string(os.PathSeparator)) && clean != filepath.Clean(base) {
		return "", false
	}
	return clean, true
}
