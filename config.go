package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type User struct {
	Name     string
	Password string
	Default  string
	None     []string
	R        []string
	W        []string
	D        []string
}

type Config struct {
	Port           int
	Paths          []string
	Allow          []string
	LogDir         string
	Title          string
	Notice         string
	ICP            string
	Police         string
	ImagePreview   bool
	FolderDownload string
	SSLCert        string
	SSLKey         string
	SessionTimeout int
	FileRemove     int
	Users          map[string]*User
	UserOrder      []string
	File           string
	Dir            string
}

func splitPaths(v string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	for _, r := range v {
		switch r {
		case '"':
			inQuote = !inQuote
		case '|':
			if inQuote {
				cur.WriteRune(r)
			} else {
				if s := strings.TrimSpace(cur.String()); s != "" {
					out = append(out, s)
				}
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		out = append(out, s)
	}
	return out
}

func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	abs, _ := filepath.Abs(path)
	cfg := &Config{
		Port:           80,
		Title:          "ModernFS",
		SessionTimeout: 30,
		FileRemove:     1,
		Users:          map[string]*User{},
		File:           abs,
		Dir:            filepath.Dir(abs),
	}

	var cur *User
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(line[1 : len(line)-1])
			u := &User{Name: name}
			cfg.Users[name] = u
			cfg.UserOrder = append(cfg.UserOrder, name)
			cur = u
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if cur != nil && (k == "password" || strings.HasPrefix(k, "rule.")) {
			switch k {
			case "password":
				cur.Password = v
			case "rule.default":
				cur.Default = strings.ToLower(v)
			case "rule.none":
				cur.None = splitPaths(v)
			case "rule.r":
				cur.R = append(cur.R, splitPaths(v)...)
			case "rule.w":
				cur.W = append(cur.W, splitPaths(v)...)
			case "rule.d":
				cur.D = append(cur.D, splitPaths(v)...)
			}
			continue
		}
		switch k {
		case "port":
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				cfg.Port = n
			}
		case "path":
			cfg.Paths = splitPaths(v)
		case "allow":
			cfg.Allow = splitPaths(v)
		case "log":
			cfg.LogDir = v
		case "html.title":
			cfg.Title = v
		case "html.notice":
			cfg.Notice = v
		case "html.icp":
			cfg.ICP = v
		case "html.police":
			cfg.Police = v
		case "image.preview":
			cfg.ImagePreview = strings.EqualFold(v, "true")
		case "folder.download":
			cfg.FolderDownload = strings.ToLower(v)
		case "ssl.cert":
			cfg.SSLCert = v
		case "ssl.key":
			cfg.SSLKey = v
		case "session.timeout":
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				cfg.SessionTimeout = n
			}
		case "file.remove":
			if n, err := strconv.Atoi(v); err == nil {
				cfg.FileRemove = n
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(cfg.Paths) == 0 {
		wd, _ := os.Getwd()
		cfg.Paths = []string{wd}
	}
	return cfg, nil
}

func (c *Config) guest() *User {
	if u, ok := c.Users["guest"]; ok {
		return u
	}
	return &User{Name: "guest"}
}

func maskFromLetters(s string) string {
	var b strings.Builder
	for _, ch := range strings.ToLower(s) {
		switch ch {
		case 'r':
			if !strings.Contains(b.String(), "R") {
				b.WriteByte('R')
			}
		case 'w':
			if !strings.Contains(b.String(), "W") {
				b.WriteByte('W')
			}
		case 'd':
			for _, c := range []string{"R", "W", "D"} {
				if !strings.Contains(b.String(), c) {
					b.WriteString(c)
				}
			}
		}
	}
	return b.String()
}

func hasMask(mask string, letter byte) bool {
	return strings.ContainsRune(strings.ToUpper(mask), rune(letter))
}

func intersectMask(a, b string) string {
	var out strings.Builder
	for _, ch := range []byte{'R', 'W', 'D'} {
		if hasMask(a, ch) && hasMask(b, ch) {
			out.WriteByte(ch)
		}
	}
	return out.String()
}

func (u *User) MaskFor(real string, isDir bool) string {
	clean := filepath.Clean(real)
	type rule struct {
		p    string
		kind byte
	}
	var rules []rule
	add := func(paths []string, kind byte) {
		for _, p := range paths {
			rp, err := filepath.Abs(strings.TrimSpace(p))
			if err != nil {
				continue
			}
			rules = append(rules, rule{p: filepath.Clean(rp), kind: kind})
		}
	}
	add(u.None, 'N')
	add(u.R, 'R')
	add(u.W, 'W')
	add(u.D, 'D')

	best := -1
	var bestKinds []byte
	for _, r := range rules {
		if clean == r.p || strings.HasPrefix(clean, r.p+string(os.PathSeparator)) {
			n := len(r.p)
			if n > best {
				best = n
				bestKinds = []byte{r.kind}
			} else if n == best {
				bestKinds = append(bestKinds, r.kind)
			}
		}
	}
	if best >= 0 {
		var mask strings.Builder
		for _, k := range bestKinds {
			if k == 'N' {
				return ""
			}
		}
		for _, k := range bestKinds {
			switch k {
			case 'R':
				if !hasMask(mask.String(), 'R') {
					mask.WriteByte('R')
				}
			case 'W':
				if !hasMask(mask.String(), 'W') {
					mask.WriteByte('W')
				}
			case 'D':
				for _, c := range []byte{'R', 'W', 'D'} {
					if !hasMask(mask.String(), c) {
						mask.WriteByte(c)
					}
				}
			}
		}
		return mask.String()
	}
	return maskFromLetters(u.Default)
}

func (c *Config) resolveUser(name string) *User {
	if u, ok := c.Users[name]; ok {
		return u
	}
	return nil
}

func (c *Config) checkPassword(name, pwd string) *User {
	u := c.resolveUser(name)
	if u == nil {
		return nil
	}
	if u.Password == pwd {
		return u
	}
	return nil
}

func (c *Config) String() string {
	return fmt.Sprintf("port=%d paths=%v users=%d", c.Port, c.Paths, len(c.Users))
}
