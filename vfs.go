package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type VFS struct {
	cfg     *Config
	root    string
	mounts  map[string]string
	allowed []string
}

type FileEntry struct {
	Name      string `json:"name"`
	Dir       bool   `json:"dir"`
	Size      int64  `json:"size"`
	Modified  string `json:"modified"`
	Icon      string `json:"icon"`
	Mask      string `json:"mask"`
	GuestMask string `json:"guestMask"`
	HasSubDir bool   `json:"hasSubDir"`
	Path      string `json:"path,omitempty"`
}

func newVFS(cfg *Config) (*VFS, error) {
	var existing []string
	for _, p := range cfg.Paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		abs = filepath.Clean(abs)
		st, err := os.Stat(abs)
		if err != nil || !st.IsDir() {
			continue
		}
		existing = append(existing, abs)
	}
	if len(existing) == 0 {
		return nil, errors.New("没有可用的共享目录")
	}

	root := filepath.Dir(existing[0])
	if root == "" {
		root = "/"
	}
	v := &VFS{cfg: cfg, root: root, mounts: map[string]string{}}
	v.allowed = append(v.allowed, root)

	for _, p := range existing {
		if v.isUnder(p, root) {
			continue
		}
		name := filepath.Base(p)
		base := name
		i := 2
		for {
			if _, ok := v.mounts[name]; !ok {
				break
			}
			name = fmt.Sprintf("%s (%d)", base, i)
			i++
		}
		v.mounts[name] = p
		v.allowed = append(v.allowed, p)
	}
	return v, nil
}

func (v *VFS) isUnder(p, root string) bool {
	if root == "/" {
		return strings.HasPrefix(p, "/")
	}
	return p == root || strings.HasPrefix(p, root+string(os.PathSeparator))
}

func cleanVP(vp string) string {
	c := path.Clean("/" + strings.TrimPrefix(vp, "/"))
	if c == "." {
		return "/"
	}
	return c
}

func (v *VFS) Resolve(vp string) (string, string, error) {
	vp = cleanVP(vp)
	if strings.Contains(vp, "..") {
		return "", "", errors.New("非法路径")
	}
	if vp == "/" {
		return v.root, vp, nil
	}
	segs := strings.SplitN(strings.TrimPrefix(vp, "/"), "/", 2)
	if target, ok := v.mounts[segs[0]]; ok {
		rest := ""
		if len(segs) == 2 {
			rest = segs[1]
		}
		real := filepath.Clean(filepath.Join(target, filepath.FromSlash(rest)))
		if !v.inAllowed(real) {
			return "", "", errors.New("非法路径")
		}
		return real, vp, nil
	}
	real := filepath.Clean(filepath.Join(v.root, filepath.FromSlash(strings.TrimPrefix(vp, "/"))))
	if !v.inAllowed(real) {
		return "", "", errors.New("非法路径")
	}
	return real, vp, nil
}

func (v *VFS) inAllowed(real string) bool {
	for _, a := range v.allowed {
		if v.isUnder(real, a) {
			return true
		}
	}
	return false
}

func (v *VFS) VirtualOf(real string) string {
	real = filepath.Clean(real)
	for name, target := range v.mounts {
		if real == target {
			return "/" + name
		}
		if v.isUnder(real, target) {
			rel, _ := filepath.Rel(target, real)
			return "/" + name + "/" + filepath.ToSlash(rel)
		}
	}
	if real == v.root {
		return "/"
	}
	if v.isUnder(real, v.root) {
		rel, _ := filepath.Rel(v.root, real)
		return "/" + filepath.ToSlash(rel)
	}
	return "/"
}

func iconFor(name string, isDir bool) string {
	if isDir {
		return "folder"
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	switch ext {
	case "txt", "log", "md", "ini", "conf", "csv":
		return "txt"
	case "jpg", "jpeg", "png", "gif", "webp", "bmp", "svg", "ico", "heic":
		return "image"
	case "mp4", "mkv", "mov", "avi", "webm", "flv", "ts", "m4v":
		return "video"
	case "mp3", "wav", "flac", "aac", "ogg", "m4a", "wma":
		return "audio"
	case "pdf":
		return "pdf"
	case "zip", "rar", "7z", "tar", "gz", "bz2", "xz":
		return "zip"
	case "exe", "msi", "apk", "bin", "iso":
		return "exe"
	case "doc", "docx", "xls", "xlsx", "ppt", "pptx":
		return "doc"
	case "go", "py", "js", "html", "htm", "css", "json", "xml", "sh", "bat", "java", "c", "cpp", "h":
		return "code"
	default:
		return "unknown"
	}
}

func (v *VFS) ListEntries(real, vp string, u, guest *User) ([]FileEntry, bool) {
	des, err := os.ReadDir(real)
	if err != nil {
		return nil, false
	}
	out := make([]FileEntry, 0, len(des))
	for _, de := range des {
		name := de.Name()
		full := filepath.Join(real, name)
		info, err := de.Info()
		if err != nil {
			continue
		}
		isDir := info.IsDir()
		mask := u.MaskFor(full, isDir)
		if mask == "" {
			continue
		}
		gmask := guest.MaskFor(full, isDir)
		entry := FileEntry{
			Name:      name,
			Dir:       isDir,
			Size:      info.Size(),
			Modified:  info.ModTime().Format("2006-01-02 15:04:05"),
			Icon:      iconFor(name, isDir),
			Mask:      mask,
			GuestMask: gmask,
		}
		if isDir {
			entry.Size = 0
			if subs, err := os.ReadDir(full); err == nil {
				for _, s := range subs {
					if s.IsDir() {
						entry.HasSubDir = true
						break
					}
				}
			}
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, true
}

func (v *VFS) Search(u, guest *User, term string, limit int) []FileEntry {
	var out []FileEntry
	term = strings.ToLower(term)
	if term == "" {
		return out
	}
	var walk func(real, vp string, depth int)
	walk = func(real, vp string, depth int) {
		if depth > 8 || len(out) >= limit {
			return
		}
		des, err := os.ReadDir(real)
		if err != nil {
			return
		}
		for _, de := range des {
			info, err := de.Info()
			if err != nil {
				continue
			}
			full := filepath.Join(real, de.Name())
			mask := u.MaskFor(full, info.IsDir())
			if mask == "" || !hasMask(mask, 'R') {
				continue
			}
			evp := path.Join(vp, de.Name())
			if strings.Contains(strings.ToLower(de.Name()), term) {
				e := FileEntry{
					Name:      de.Name(),
					Dir:       info.IsDir(),
					Size:      info.Size(),
					Modified:  info.ModTime().Format("2006-01-02 15:04:05"),
					Icon:      iconFor(de.Name(), info.IsDir()),
					Mask:      mask,
					GuestMask: guest.MaskFor(full, info.IsDir()),
					Path:      evp,
				}
				if info.IsDir() {
					e.Size = 0
				}
				out = append(out, e)
				if len(out) >= limit {
					return
				}
			}
			if info.IsDir() {
				walk(full, evp, depth+1)
			}
		}
	}
	walk(v.root, "/", 0)
	// 挂载目录不在 root 下，单独搜索
	for name, target := range v.mounts {
		walk(target, "/"+name, 0)
	}
	return out
}

func (v *VFS) isRootOrAncestor(real string) bool {
	clean := filepath.Clean(real)
	if clean == v.root {
		return true
	}
	if strings.HasPrefix(v.root, clean+string(os.PathSeparator)) {
		return true
	}
	for _, m := range v.mounts {
		if clean == m || strings.HasPrefix(m, clean+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}
