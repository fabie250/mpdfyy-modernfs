package main

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/net/webdav"
)

type davFS struct {
	s *Server
	u *User
}

func (d *davFS) resolve(name string) (string, string, error) {
	vp := path.Clean("/" + strings.TrimPrefix(name, "/"))
	real, vp, err := d.s.vfs.Resolve(vp)
	if err != nil {
		return "", "", os.ErrInvalid
	}
	return real, vp, nil
}

func (d *davFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	real, _, err := d.resolve(name)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(real)
	if err != nil {
		return nil, err
	}
	if d.u.MaskFor(real, st.IsDir()) == "" {
		return nil, os.ErrNotExist
	}
	return st, nil
}

func (d *davFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	real, _, err := d.resolve(name)
	if err != nil {
		return nil, err
	}
	st, statErr := os.Stat(real)
	isDir := statErr == nil && st.IsDir()
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0 {
		if !hasMask(d.u.MaskFor(real, isDir), 'W') {
			return nil, os.ErrPermission
		}
	} else {
		if !hasMask(d.u.MaskFor(real, isDir), 'R') {
			return nil, os.ErrPermission
		}
	}
	f, err := os.OpenFile(real, flag, perm)
	if err != nil {
		return nil, err
	}
	return &permFile{File: f, u: d.u, real: real, s: d.s}, nil
}

func (d *davFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	real, _, err := d.resolve(name)
	if err != nil {
		return err
	}
	if !hasMask(d.u.MaskFor(filepath.Dir(real), true), 'W') {
		return os.ErrPermission
	}
	return os.Mkdir(real, perm)
}

func (d *davFS) RemoveAll(ctx context.Context, name string) error {
	real, _, err := d.resolve(name)
	if err != nil {
		return err
	}
	if d.s.vfs.isRootOrAncestor(real) {
		return os.ErrPermission
	}
	if !hasMask(d.u.MaskFor(filepath.Dir(real), true), 'D') {
		return os.ErrPermission
	}
	return os.RemoveAll(real)
}

func (d *davFS) Rename(ctx context.Context, oldName, newName string) error {
	oldReal, _, err := d.resolve(oldName)
	if err != nil {
		return err
	}
	newReal, _, err := d.resolve(newName)
	if err != nil {
		return err
	}
	if d.s.vfs.isRootOrAncestor(oldReal) || d.s.vfs.isRootOrAncestor(newReal) {
		return os.ErrPermission
	}
	if !hasMask(d.u.MaskFor(filepath.Dir(oldReal), true), 'D') {
		return os.ErrPermission
	}
	if !hasMask(d.u.MaskFor(filepath.Dir(newReal), true), 'W') {
		return os.ErrPermission
	}
	return os.Rename(oldReal, newReal)
}

type permFile struct {
	*os.File
	u    *User
	real string
	s    *Server
}

func (p *permFile) Readdir(count int) ([]fs.FileInfo, error) {
	infos, err := p.File.Readdir(count)
	if err != nil {
		return nil, err
	}
	out := make([]fs.FileInfo, 0, len(infos))
	for _, info := range infos {
		full := filepath.Join(p.real, info.Name())
		if p.u.MaskFor(full, info.IsDir()) == "" {
			continue
		}
		out = append(out, info)
	}
	return out, nil
}

var errUnsupported = errors.New("unsupported")

func (s *Server) webdavHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := s.auth.Current(r)
		if u.Name == "guest" && s.cfg.Users["guest"] == nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="ModernFS"`)
			http.Error(w, "需要登录", http.StatusUnauthorized)
			return
		}
		h := &webdav.Handler{
			Prefix:     "/webdav",
			FileSystem: &davFS{s: s, u: u},
			LockSystem: webdav.NewMemLS(),
			Logger:     func(req *http.Request, err error) {},
		}
		h.ServeHTTP(w, r)
	})
}
