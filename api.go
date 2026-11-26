package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"golang.org/x/image/draw"
)

type Server struct {
	cfg      *Config
	vfs      *VFS
	auth     *Auth
	trash    *Trash
	shares   *ShareStore
	secure   bool
	thumbDir string
}

func jsonErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) need(w http.ResponseWriter, u *User, real string, isDir bool, letter byte) bool {
	mask := u.MaskFor(real, isDir)
	if !hasMask(mask, letter) {
		jsonErr(w, 403, "没有权限")
		return false
	}
	return true
}

func param(r *http.Request, key string) string {
	if v := r.FormValue(key); v != "" {
		return v
	}
	if v := r.URL.Query().Get(key); v != "" {
		return v
	}
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
		if b, err := io.ReadAll(io.LimitReader(r.Body, 4<<20)); err == nil {
			if vals, err := url.ParseQuery(string(b)); err == nil {
				return vals.Get(key)
			}
		}
	}
	return ""
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request, u *User) {
	vp := r.URL.Query().Get("filepath")
	real, vp, err := s.vfs.Resolve(vp)
	if err != nil {
		jsonErr(w, 400, "参数无效")
		return
	}
	st, err := os.Stat(real)
	if err != nil || !st.IsDir() {
		jsonErr(w, 404, "目录不存在")
		return
	}
	cur := u.MaskFor(real, true)
	guest := s.cfg.guest()
	var files []FileEntry
	if cur == "" || !hasMask(cur, 'R') {
		if vp == "/" {
			files = filterRoot(s.vfs, real, vp, u, guest)
		} else {
			files = []FileEntry{}
		}
	} else {
		files, _ = s.vfs.ListEntries(real, vp, u, guest)
	}
	if files == nil {
		files = []FileEntry{}
	}
	jsonOK(w, map[string]any{"virtualRoot": 0, "currentMask": cur, "files": files})
}

func filterRoot(v *VFS, real, vp string, u, guest *User) []FileEntry {
	all, _ := v.ListEntries(real, vp, &User{Default: "rwd"}, guest)
	out := make([]FileEntry, 0, len(all))
	for _, e := range all {
		if u.MaskFor(filepath.Join(real, e.Name), e.Dir) != "" {
			e.Mask = u.MaskFor(filepath.Join(real, e.Name), e.Dir)
			out = append(out, e)
		}
	}
	return out
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request, u *User) {
	term := r.URL.Query().Get("str")
	guest := s.cfg.guest()
	files := s.vfs.Search(u, guest, term, 600)
	if files == nil {
		files = []FileEntry{}
	}
	jsonOK(w, map[string]any{"virtualRoot": 0, "currentMask": "R", "files": files})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		u, _ := s.auth.Current(r)
		authed := u.Name != "guest" || (s.cfg.Users["guest"] != nil && u.Name == "guest" && s.auth.cookieToken(r) != "")
		jsonOK(w, map[string]any{"user": u.Name, "authenticated": authed})
	case http.MethodPost:
		_ = r.ParseMultipartForm(8 << 20)
		name := strings.TrimSpace(r.FormValue("user"))
		pwd := r.FormValue("pwd")
		if name == "" || pwd == "" {
			jsonErr(w, 400, "用户名或密码为空")
			return
		}
		user := s.cfg.checkPassword(name, pwd)
		if user == nil {
			jsonErr(w, 401, "用户名或密码错误")
			return
		}
		tok, exp := s.auth.issue(user.Name)
		s.auth.setCookie(w, tok, exp, s.secure)
		http.SetCookie(w, &http.Cookie{Name: "user", Value: url.QueryEscape(user.Name), Path: "/", Expires: exp, SameSite: http.SameSiteLaxMode, Secure: s.secure})
		w.WriteHeader(http.StatusCreated)
		jsonOK(w, map[string]any{"user": user.Name})
	case http.MethodDelete:
		if tok := s.auth.cookieToken(r); tok != "" {
			s.auth.revoke(tok)
		}
		s.auth.clearCookie(w, s.secure)
		jsonOK(w, map[string]string{"status": "ok"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func cleanRel(name string) (string, bool) {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimLeft(name, "/")
	parts := strings.Split(name, "/")
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return "", false
		}
	}
	return strings.Join(parts, "/"), true
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request, u *User) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	folder := r.FormValue("folder")
	if folder == "" {
		folder = "/"
	}
	realDir, vpDir, err := s.vfs.Resolve(folder)
	if err != nil {
		jsonErr(w, 400, "非法路径")
		return
	}
	st, err := os.Stat(realDir)
	if err != nil || !st.IsDir() {
		jsonErr(w, 404, "目录不存在")
		return
	}
	if !s.need(w, u, realDir, true, 'W') {
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		jsonErr(w, 400, "上传解析失败")
		return
	}
	form := r.MultipartForm
	rel := r.FormValue("path")
	count := 0
	for _, fhs := range form.File {
		for _, fh := range fhs {
			name := rel
			if name == "" {
				name = fh.Filename
			}
			cleaned, ok := cleanRel(name)
			if !ok {
				jsonErr(w, 400, "非法文件名")
				return
			}
			target := filepath.Join(realDir, filepath.FromSlash(cleaned))
			if !s.vfs.inAllowed(target) {
				jsonErr(w, 400, "非法路径")
				return
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				jsonErr(w, 500, "创建目录失败")
				return
			}
			src, err := fh.Open()
			if err != nil {
				jsonErr(w, 500, "读取上传文件失败")
				return
			}
			tmp := target + ".part"
			out, err := os.Create(tmp)
			if err != nil {
				src.Close()
				jsonErr(w, 500, "写入失败")
				return
			}
			_, cerr := io.Copy(out, src)
			out.Close()
			src.Close()
			if cerr != nil {
				_ = os.Remove(tmp)
				jsonErr(w, 500, "写入失败")
				return
			}
			_ = os.Chmod(tmp, 0644)
			if err := os.Rename(tmp, target); err != nil {
				_ = os.Remove(tmp)
				jsonErr(w, 500, "保存失败")
				return
			}
			count++
			rel = ""
			_ = vpDir
		}
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]any{"uploaded": count})
}

func (s *Server) handleUpload2(w http.ResponseWriter, r *http.Request, u *User) {
	// 目录上传：先按 paths[] 建目录，再走普通上传逻辑
	_ = r.ParseMultipartForm(64 << 20)
	folder := r.FormValue("folder")
	realDir, _, err := s.vfs.Resolve(folder)
	if err != nil {
		jsonErr(w, 400, "非法路径")
		return
	}
	if !s.need(w, u, realDir, true, 'W') {
		return
	}
	for _, p := range r.MultipartForm.Value["paths[]"] {
		cleaned, ok := cleanRel(p)
		if !ok {
			continue
		}
		_ = os.MkdirAll(filepath.Join(realDir, filepath.FromSlash(cleaned)), 0755)
	}
	s.handleUploadForm(w, r, u, realDir)
}

func (s *Server) handleUploadForm(w http.ResponseWriter, r *http.Request, u *User, realDir string) {
	for _, fhs := range r.MultipartForm.File {
		for _, fh := range fhs {
			cleaned, ok := cleanRel(fh.Filename)
			if !ok {
				jsonErr(w, 400, "非法文件名")
				return
			}
			target := filepath.Join(realDir, filepath.FromSlash(cleaned))
			if !s.vfs.inAllowed(target) {
				jsonErr(w, 400, "非法路径")
				return
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				jsonErr(w, 500, "创建目录失败")
				return
			}
			src, err := fh.Open()
			if err != nil {
				jsonErr(w, 500, "读取失败")
				return
			}
			out, err := os.Create(target)
			if err != nil {
				src.Close()
				jsonErr(w, 500, "写入失败")
				return
			}
			_, _ = io.Copy(out, src)
			out.Close()
			src.Close()
		}
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (s *Server) handleNewDir(w http.ResponseWriter, r *http.Request, u *User) {
	vp := firstNonEmpty(r.FormValue("filepath"), r.URL.Query().Get("filepath"))
	real, _, err := s.vfs.Resolve(vp)
	if err != nil {
		jsonErr(w, 400, "非法路径")
		return
	}
	parent := filepath.Dir(real)
	if !s.need(w, u, parent, true, 'W') {
		return
	}
	if err := os.MkdirAll(real, 0755); err != nil {
		jsonErr(w, 500, "创建失败")
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (s *Server) handleNewText(w http.ResponseWriter, r *http.Request, u *User) {
	dirVP := r.FormValue("filepath")
	title := strings.TrimSpace(r.FormValue("title"))
	text := r.FormValue("text")
	realDir, _, err := s.vfs.Resolve(dirVP)
	if err != nil {
		jsonErr(w, 400, "非法路径")
		return
	}
	if !s.need(w, u, realDir, true, 'W') {
		return
	}
	name, ok := cleanRel(title)
	if !ok {
		jsonErr(w, 400, "非法文件名")
		return
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if err := os.WriteFile(filepath.Join(realDir, filepath.FromSlash(name)), []byte(text), 0644); err != nil {
		jsonErr(w, 500, "写入失败")
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (s *Server) handleUpdateText(w http.ResponseWriter, r *http.Request, u *User) {
	dirVP := r.FormValue("filepath")
	name, ok := cleanRel(r.FormValue("filename"))
	if !ok {
		jsonErr(w, 400, "非法文件名")
		return
	}
	realDir, _, err := s.vfs.Resolve(dirVP)
	if err != nil {
		jsonErr(w, 400, "非法路径")
		return
	}
	target := filepath.Join(realDir, filepath.FromSlash(name))
	if !s.need(w, u, target, false, 'W') {
		return
	}
	text := strings.ReplaceAll(r.FormValue("text"), "\r\n", "\n")
	if err := os.WriteFile(target, []byte(text), 0644); err != nil {
		jsonErr(w, 500, "写入失败")
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request, u *User) {
	oldVP := strings.TrimSpace(param(r, "old"))
	newVP := strings.TrimSpace(param(r, "new"))
	if oldVP == "" || newVP == "" || oldVP == "/" || newVP == "/" {
		jsonErr(w, 400, "缺少路径")
		return
	}
	oldReal, _, err := s.vfs.Resolve(oldVP)
	if err != nil {
		jsonErr(w, 400, "非法路径")
		return
	}
	newReal, _, err := s.vfs.Resolve(newVP)
	if err != nil {
		jsonErr(w, 400, "非法路径")
		return
	}
	if s.vfs.isRootOrAncestor(oldReal) || s.vfs.isRootOrAncestor(newReal) {
		jsonErr(w, 403, "禁止操作共享根目录")
		return
	}
	if !s.need(w, u, filepath.Dir(oldReal), true, 'D') {
		return
	}
	if !s.need(w, u, filepath.Dir(newReal), true, 'W') {
		return
	}
	if err := os.Rename(oldReal, newReal); err != nil {
		jsonErr(w, 500, "重命名失败")
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (s *Server) handleRemove(w http.ResponseWriter, r *http.Request, u *User) {
	vp := strings.TrimSpace(param(r, "filepath"))
	if vp == "" || vp == "/" {
		jsonErr(w, 400, "缺少文件路径")
		return
	}
	real, vp, err := s.vfs.Resolve(vp)
	if err != nil {
		jsonErr(w, 400, "非法路径")
		return
	}
	if s.vfs.isRootOrAncestor(real) {
		jsonErr(w, 403, "禁止删除共享根目录或其上级目录")
		return
	}
	if !s.need(w, u, filepath.Dir(real), true, 'D') {
		return
	}
	info, err := os.Lstat(real)
	if err != nil {
		jsonErr(w, 404, "文件不存在")
		return
	}
	// 禁止删除正在运行的配置与程序自身
	absSelf, _ := os.Executable()
	if filepath.Clean(real) == filepath.Clean(s.cfg.File) || filepath.Clean(real) == filepath.Clean(absSelf) {
		jsonErr(w, 403, "该文件受保护，禁止删除")
		return
	}
	if s.cfg.FileRemove == 3 {
		err = os.RemoveAll(real)
	} else {
		err = s.trash.Add(real, vp, u.Name)
	}
	if err != nil {
		jsonErr(w, 500, "删除失败")
		return
	}
	_ = info
	jsonOK(w, map[string]string{"status": "ok"})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func contentDisposition(name string, inline bool) string {
	typ := "attachment"
	if inline {
		typ = "inline"
	}
	ascii := strings.Map(func(r rune) rune {
		if r < 32 || r > 126 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	return fmt.Sprintf("%s; filename=\"%s\"; filename*=UTF-8''%s", typ, ascii, url.PathEscape(name))
}

func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, real string, inline bool) {
	f, err := os.Open(real)
	if err != nil {
		jsonErr(w, 404, "文件不存在")
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		jsonErr(w, 500, "读取失败")
		return
	}
	w.Header().Set("Content-Disposition", contentDisposition(st.Name(), inline))
	if inline {
		if ct := detectType(real); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
	}
	http.ServeContent(w, r, st.Name(), st.ModTime(), f)
}

func detectType(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mkv":
		return "video/x-matroska"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".flac":
		return "audio/flac"
	case ".ogg":
		return "audio/ogg"
	case ".m4a":
		return "audio/mp4"
	case ".pdf":
		return "application/pdf"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".txt", ".md", ".log", ".ini", ".conf", ".csv", ".json", ".xml", ".go", ".py", ".js", ".html", ".css", ".sh":
		return "text/plain; charset=utf-8"
	}
	return ""
}

func (s *Server) handleShared(w http.ResponseWriter, r *http.Request, u *User) {
	vp := strings.TrimPrefix(r.URL.Path, "/chfs/shared")
	if vp == "" {
		vp = r.URL.Query().Get("file")
	}
	real, _, err := s.vfs.Resolve(vp)
	if err != nil {
		jsonErr(w, 400, "非法路径")
		return
	}
	st, err := os.Stat(real)
	if err != nil {
		jsonErr(w, 404, "文件不存在")
		return
	}
	if !s.need(w, u, real, st.IsDir(), 'R') {
		return
	}
	if st.IsDir() {
		s.zipDir(w, r, real)
		return
	}
	s.serveFile(w, r, real, false)
}

func (s *Server) handleRaw(w http.ResponseWriter, r *http.Request, u *User) {
	vp := r.URL.Query().Get("file")
	real, _, err := s.vfs.Resolve(vp)
	if err != nil {
		jsonErr(w, 400, "非法路径")
		return
	}
	st, err := os.Stat(real)
	if err != nil || st.IsDir() {
		jsonErr(w, 404, "文件不存在")
		return
	}
	if !s.need(w, u, real, false, 'R') {
		return
	}
	s.serveFile(w, r, real, true)
}

func (s *Server) zipDir(w http.ResponseWriter, r *http.Request, real string) {
	if s.cfg.FolderDownload == "disable" {
		jsonErr(w, 403, "目录下载已禁用")
		return
	}
	base := filepath.Base(real)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", contentDisposition(base+".zip", false))
	zw := zip.NewWriter(w)
	defer zw.Close()
	root := real
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil || rel == "." {
			return nil
		}
		if s.cfg.FolderDownload == "leaf" && d.IsDir() && p != root {
			ents, e := os.ReadDir(p)
			if e == nil {
				for _, en := range ents {
					if en.IsDir() {
						return filepath.SkipDir
					}
				}
			}
		}
		hdr, err := zip.FileInfoHeader(infoOf(d))
		if err != nil {
			return nil
		}
		hdr.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			hdr.Name += "/"
			hdr.Method = zip.Store
		} else {
			hdr.Method = zip.Deflate
		}
		wr, err := zw.CreateHeader(hdr)
		if err != nil || d.IsDir() {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		_, _ = io.Copy(wr, f)
		f.Close()
		return nil
	})
}

func infoOf(d os.DirEntry) os.FileInfo {
	i, err := d.Info()
	if err != nil {
		return fakeInfo{name: d.Name(), dir: d.IsDir()}
	}
	return i
}

type fakeInfo struct {
	name string
	dir  bool
}

func (f fakeInfo) Name() string { return f.name }
func (f fakeInfo) Size() int64  { return 0 }
func (f fakeInfo) Mode() os.FileMode {
	if f.dir {
		return os.ModeDir | 0755
	}
	return 0644
}
func (f fakeInfo) ModTime() time.Time { return time.Now() }
func (f fakeInfo) IsDir() bool        { return f.dir }
func (f fakeInfo) Sys() any           { return nil }

func (s *Server) handleDownloadDir(w http.ResponseWriter, r *http.Request, u *User) {
	vp := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/chfs/downloaddir"), ".zip")
	real, _, err := s.vfs.Resolve(vp)
	if err != nil {
		jsonErr(w, 400, "非法路径")
		return
	}
	st, err := os.Stat(real)
	if err != nil || !st.IsDir() {
		jsonErr(w, 404, "目录不存在")
		return
	}
	if !s.need(w, u, real, true, 'R') {
		return
	}
	s.zipDir(w, r, real)
}

func (s *Server) handleThumb(w http.ResponseWriter, r *http.Request, u *User) {
	vp := r.URL.Query().Get("file")
	real, _, err := s.vfs.Resolve(vp)
	if err != nil {
		jsonErr(w, 400, "非法路径")
		return
	}
	st, err := os.Stat(real)
	if err != nil || st.IsDir() {
		jsonErr(w, 404, "文件不存在")
		return
	}
	if !s.need(w, u, real, false, 'R') {
		return
	}
	ext := strings.ToLower(filepath.Ext(real))
	if ext == ".svg" {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		http.ServeFile(w, r, real)
		return
	}
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
	default:
		jsonErr(w, 415, "不支持预览")
		return
	}
	want, _ := strconv.Atoi(r.URL.Query().Get("w"))
	if want <= 0 || want > 720 {
		want = 320
	}
	key := sha1.Sum([]byte(fmt.Sprintf("%s|%d|%d|%d", real, st.Size(), st.ModTime().UnixNano(), want)))
	cache := filepath.Join(s.thumbDir, hex.EncodeToString(key[:])+".jpg")
	if b, err := os.ReadFile(cache); err == nil {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		_, _ = w.Write(b)
		return
	}
	f, err := os.Open(real)
	if err != nil {
		jsonErr(w, 500, "读取失败")
		return
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		jsonErr(w, 415, "图片解码失败")
		return
	}
	b := img.Bounds()
	h := b.Dy() * want / b.Dx()
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, want, h))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 82}); err != nil {
		jsonErr(w, 500, "编码失败")
		return
	}
	_ = os.WriteFile(cache, buf.Bytes(), 0644)
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) handleQR(w http.ResponseWriter, r *http.Request, u *User) {
	text := r.URL.Query().Get("text")
	if text == "" {
		jsonErr(w, 400, "缺少内容")
		return
	}
	png, err := qrcode.Encode(text, qrcode.Medium, 256)
	if err != nil {
		jsonErr(w, 500, "生成失败")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(png)
}

func (s *Server) handleShareCreate(w http.ResponseWriter, r *http.Request, u *User) {
	_ = r.ParseMultipartForm(8 << 20)
	vp := r.FormValue("filepath")
	real, vp, err := s.vfs.Resolve(vp)
	if err != nil {
		jsonErr(w, 400, "非法路径")
		return
	}
	st, err := os.Stat(real)
	if err != nil {
		jsonErr(w, 404, "文件不存在")
		return
	}
	if !s.need(w, u, real, st.IsDir(), 'R') {
		return
	}
	ttl := parseTTL(r.FormValue("ttl"))
	pwd := r.FormValue("password")
	sh := s.shares.Create(vp, real, u.Name, st.Name(), st.IsDir(), ttl, pwd)
	jsonOK(w, map[string]any{"token": sh.Token, "url": "/s/" + sh.Token, "expires": sh.ExpiresAt})
}

func parseTTL(v string) time.Duration {
	switch v {
	case "1h":
		return time.Hour
	case "1d":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return 0
	}
}

func (s *Server) handleShareList(w http.ResponseWriter, r *http.Request, u *User) {
	if u.Name == "guest" {
		jsonErr(w, 401, "请先登录")
		return
	}
	items := s.shares.List(u.Name)
	if items == nil {
		items = []*Share{}
	}
	jsonOK(w, items)
}

func (s *Server) handleShareDelete(w http.ResponseWriter, r *http.Request, u *User) {
	if u.Name == "guest" {
		jsonErr(w, 401, "请先登录")
		return
	}
	_ = r.ParseMultipartForm(8 << 20)
	tok := param(r, "token")
	if !s.shares.Delete(u.Name, tok) {
		jsonErr(w, 404, "分享不存在")
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (s *Server) handlePublicShare(w http.ResponseWriter, r *http.Request) {
	tok := strings.TrimPrefix(r.URL.Path, "/s/")
	sh, ok := s.shares.Get(tok)
	if !ok {
		http.Error(w, "分享链接不存在或已过期", http.StatusNotFound)
		return
	}
	if sh.Password != "" {
		pwd := r.URL.Query().Get("pwd")
		if pwd == "" && r.Method == http.MethodPost {
			_ = r.ParseForm()
			pwd = r.FormValue("pwd")
		}
		if pwd != sh.Password {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, sharePasswordPage(tok))
			return
		}
	}
	real := sh.Real
	st, err := os.Stat(real)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.shares.BumpDownload(tok)
	if st.IsDir() {
		if s.cfg.FolderDownload == "disable" {
			http.Error(w, "目录下载已禁用", http.StatusForbidden)
			return
		}
		s.zipDir(w, r, real)
		return
	}
	s.serveFile(w, r, real, false)
}

func sharePasswordPage(token string) string {
	return `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>需要密码</title>
<style>body{background:#0b1017;color:#e6edf7;font-family:system-ui;display:flex;height:100vh;align-items:center;justify-content:center}
.box{background:#141b26;padding:28px 32px;border-radius:14px;border:1px solid #24303f;width:320px}
h2{font-size:16px;margin:0 0 16px}input{width:100%;padding:10px;border-radius:8px;border:1px solid #2b3a4d;background:#0c1219;color:#fff;box-sizing:border-box}
button{margin-top:12px;width:100%;padding:10px;border:0;border-radius:8px;background:#2f81f7;color:#fff;font-size:14px}</style>
<div class="box"><h2>该分享需要密码</h2><form method="post"><input name="pwd" type="password" placeholder="访问密码" autofocus><button>进入</form></div></html>`
}

func (s *Server) handleTrashList(w http.ResponseWriter, r *http.Request, u *User) {
	if u.Name == "guest" {
		jsonErr(w, 401, "请先登录")
		return
	}
	jsonOK(w, map[string]any{"items": s.trash.List()})
}

func (s *Server) handleTrashAction(w http.ResponseWriter, r *http.Request, u *User) {
	if u.Name == "guest" {
		jsonErr(w, 401, "请先登录")
		return
	}
	_ = r.ParseMultipartForm(8 << 20)
	id := r.FormValue("id")
	action := r.FormValue("action")
	switch action {
	case "restore":
		it, err := s.trash.Restore(id)
		if err != nil {
			jsonErr(w, 500, err.Error())
			return
		}
		jsonOK(w, it)
	case "purge":
		if err := s.trash.Purge(id); err != nil {
			jsonErr(w, 500, err.Error())
			return
		}
		jsonOK(w, map[string]string{"status": "ok"})
	case "empty":
		s.trash.Empty()
		jsonOK(w, map[string]string{"status": "ok"})
	default:
		jsonErr(w, 400, "未知操作")
	}
}

func (s *Server) handleExist(w http.ResponseWriter, r *http.Request, u *User) {
	vp := r.URL.Query().Get("file")
	real, _, err := s.vfs.Resolve(vp)
	if err != nil {
		jsonOK(w, map[string]bool{"exist": false})
		return
	}
	_, err = os.Stat(real)
	jsonOK(w, map[string]bool{"exist": err == nil})
}

func (s *Server) handleStat(w http.ResponseWriter, r *http.Request, u *User) {
	vp := r.URL.Query().Get("file")
	real, vp, err := s.vfs.Resolve(vp)
	if err != nil {
		jsonErr(w, 400, "非法路径")
		return
	}
	st, err := os.Stat(real)
	if err != nil {
		jsonErr(w, 404, "文件不存在")
		return
	}
	mask := u.MaskFor(real, st.IsDir())
	if mask == "" {
		jsonErr(w, 403, "没有权限")
		return
	}
	jsonOK(w, map[string]any{
		"name": st.Name(), "virtual": vp, "dir": st.IsDir(), "size": st.Size(),
		"modified": st.ModTime().Format("2006-01-02 15:04:05"), "mask": mask,
	})
}

func (s *Server) handleText(w http.ResponseWriter, r *http.Request, u *User) {
	vp := r.URL.Query().Get("file")
	real, _, err := s.vfs.Resolve(vp)
	if err != nil {
		jsonErr(w, 400, "非法路径")
		return
	}
	if !s.need(w, u, real, false, 'R') {
		return
	}
	ext := strings.ToLower(filepath.Ext(real))
	switch ext {
	case ".txt", ".md", ".log", ".ini", ".conf", ".csv", ".json", ".xml", ".go", ".py", ".js", ".html", ".css", ".sh", ".bat", ".yml", ".yaml":
	default:
		jsonErr(w, 415, "不支持文本预览")
		return
	}
	b, err := os.ReadFile(real)
	if err != nil {
		jsonErr(w, 500, "读取失败")
		return
	}
	if len(b) > 2<<20 {
		b = b[:2<<20]
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(b)
}

func pathJoinVP(base, name string) string {
	return path.Join("/", strings.TrimPrefix(base, "/"), name)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request, u *User) {
	jsonOK(w, map[string]any{
		"title":   s.cfg.Title,
		"notice":  s.cfg.Notice,
		"icp":     s.cfg.ICP,
		"police":  s.cfg.Police,
		"version": version,
		"secure":  s.secure,
		"webdav":  "/webdav",
		"user":    u.Name,
		"guest":   u.Name == "guest",
		"remove":  s.cfg.FileRemove,
	})
}
