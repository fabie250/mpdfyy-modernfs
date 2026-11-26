package main

import (
	"embed"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed assets/*
var assetsFS embed.FS

const version = "1.0.0"

type accessLog struct {
	mu   sync.Mutex
	f    *os.File
	path string
}

func newAccessLog(dir string) *accessLog {
	if dir == "" {
		return &accessLog{}
	}
	_ = os.MkdirAll(dir, 0755)
	p := filepath.Join(dir, "modernfs-"+time.Now().Format("20060102")+".log")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("[warn] 无法打开日志文件:", err)
		return &accessLog{}
	}
	return &accessLog{f: f, path: p}
}

func (a *accessLog) write(line string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	fmt.Println(line)
	if a.f != nil {
		_, _ = a.f.WriteString(line + "\n")
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (s *Server) middleware(next http.Handler, log *accessLog) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if !s.allowIP(r) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		if log != nil && !strings.HasPrefix(r.URL.Path, "/asset/") {
			ip := clientIP(r)
			log.write(fmt.Sprintf("%s %s \"%s %s\" %d %dms", time.Now().Format("2006-01-02 15:04:05"), ip, r.Method, r.URL.RequestURI(), sw.status, time.Since(start).Milliseconds()))
		}
	})
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return v
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) allowIP(r *http.Request) bool {
	if len(s.cfg.Allow) == 0 {
		return true
	}
	ipStr := clientIP(r)
	ip := net.ParseIP(ipStr)
	for _, rule := range s.cfg.Allow {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		if strings.HasSuffix(rule, "*") {
			if strings.HasPrefix(ipStr, strings.TrimSuffix(rule, "*")) {
				return true
			}
			continue
		}
		if _, cidr, err := net.ParseCIDR(rule); err == nil {
			if ip != nil && cidr.Contains(ip) {
				return true
			}
			continue
		}
		if ipStr == rule {
			return true
		}
	}
	return false
}

func main() {
	var cfgFile string
	var portOverride int
	flag.StringVar(&cfgFile, "file", "chfs.ini", "配置文件路径")
	flag.IntVar(&portOverride, "port", 0, "覆盖监听端口")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "ModernFS 文件共享服务 v%s\n\n", version)
		fmt.Fprintln(out, "用法：")
		fmt.Fprintln(out, "  modernfs-linux-amd64 [-file 配置文件] [-port 端口]")
		fmt.Fprintln(out, "\n命令行参数：")
		flag.PrintDefaults()
		fmt.Fprintln(out, `
配置文件常用项：
  port=443                    监听端口
  path=/srv/chfs/public        共享目录，多个目录用 | 分隔
  allow=                      IP 白名单，留空表示不限制
  log=/var/log/modernfs      操作日志目录
  html.title=文件空间         网页标题
  html.notice=公告文字        顶部公告
  html.icp=备案号             ICP 备案号
  html.police=公安备案号      公安联网备案号
  image.preview=true          是否开启图片预览
  folder.download=enable      目录下载：enable/leaf/disable
  ssl.cert=证书路径           HTTPS 证书
  ssl.key=私钥路径            HTTPS 私钥
  session.timeout=60          会话时长，单位分钟
  file.remove=3               删除模式：1 系统回收站，2 专属回收站，3 永久删除

账号规则：
  [账号名]
  password=密码
  rule.default=rwd            默认权限：r 只读，w 可写，d 完全权限
  rule.r=目录                 只读目录
  rule.w=目录                 可写目录
  rule.d=目录                 完全权限目录

示例：
  ./modernfs-linux-amd64 -file=modernfs.ini
  ./modernfs-linux-amd64 -file=modernfs.ini -port=8080`)
	}
	flag.Parse()

	cfg, err := loadConfig(cfgFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "读取配置失败:", err)
		os.Exit(1)
	}
	if portOverride > 0 {
		cfg.Port = portOverride
	}
	vfs, err := newVFS(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化共享目录失败:", err)
		os.Exit(1)
	}

	thumbDir := filepath.Join(os.TempDir(), "modernfs-thumbs")
	_ = os.MkdirAll(thumbDir, 0755)

	s := &Server{
		cfg:      cfg,
		vfs:      vfs,
		auth:     newAuth(cfg),
		trash:    newTrash(cfg.Dir),
		shares:   newShareStore(cfg.Dir),
		thumbDir: thumbDir,
	}
	s.secure = cfg.SSLCert != "" && cfg.SSLKey != ""

	log := newAccessLog(cfg.LogDir)
	if log.f != nil {
		defer log.f.Close()
	}

	mux := http.NewServeMux()

	serveAsset := func(name string, ct string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			b, err := assetsFS.ReadFile("assets/" + name)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", ct)
			w.Header().Set("Cache-Control", "no-cache, must-revalidate")
			_, _ = w.Write(b)
		}
	}
	mux.HandleFunc("/asset/app.css", serveAsset("app.css", "text/css; charset=utf-8"))
	mux.HandleFunc("/asset/app.js", serveAsset("app.js", "application/javascript; charset=utf-8"))
	mux.HandleFunc("/asset/logo.svg", serveAsset("logo.svg", "image/svg+xml"))
	mux.HandleFunc("/favicon.ico", serveAsset("logo.svg", "image/svg+xml"))
	vendor := map[string]string{
		"docx-preview.min.js": "application/javascript; charset=utf-8",
		"jszip.min.js":        "application/javascript; charset=utf-8",
		"xlsx.full.min.js":    "application/javascript; charset=utf-8",
	}
	mux.HandleFunc("/asset/vendor/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/asset/vendor/")
		ct, ok := vendor[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		b, err := assetsFS.ReadFile("assets/vendor/" + name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = w.Write(b)
	})

	userOf := func(r *http.Request) *User {
		u, _ := s.auth.Current(r)
		return u
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		b, _ := assetsFS.ReadFile("assets/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		_, _ = w.Write(b)
	})

	mux.HandleFunc("/chfs/config", func(w http.ResponseWriter, r *http.Request) {
		s.handleConfig(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/files", func(w http.ResponseWriter, r *http.Request) {
		s.handleFiles(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/search", func(w http.ResponseWriter, r *http.Request) {
		s.handleSearch(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/session", s.handleSession)
	mux.HandleFunc("/chfs/upload", func(w http.ResponseWriter, r *http.Request) {
		s.handleUpload(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/upload2", func(w http.ResponseWriter, r *http.Request) {
		s.handleUpload2(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/newdir", func(w http.ResponseWriter, r *http.Request) {
		s.handleNewDir(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/newtext", func(w http.ResponseWriter, r *http.Request) {
		s.handleNewText(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/updatetext", func(w http.ResponseWriter, r *http.Request) {
		s.handleUpdateText(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/rename", func(w http.ResponseWriter, r *http.Request) {
		s.handleRename(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/rmfiles", func(w http.ResponseWriter, r *http.Request) {
		s.handleRemove(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/shared", func(w http.ResponseWriter, r *http.Request) {
		s.handleShared(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/shared/", func(w http.ResponseWriter, r *http.Request) {
		s.handleShared(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/raw", func(w http.ResponseWriter, r *http.Request) {
		s.handleRaw(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/downloaddir", func(w http.ResponseWriter, r *http.Request) {
		s.handleDownloadDir(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/downloaddir/", func(w http.ResponseWriter, r *http.Request) {
		s.handleDownloadDir(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/thumb", func(w http.ResponseWriter, r *http.Request) {
		s.handleThumb(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/text", func(w http.ResponseWriter, r *http.Request) {
		s.handleText(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/qr", func(w http.ResponseWriter, r *http.Request) {
		s.handleQR(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/share", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			s.handleShareCreate(w, r, userOf(r))
		case http.MethodDelete:
			s.handleShareDelete(w, r, userOf(r))
		default:
			s.handleShareList(w, r, userOf(r))
		}
	})
	mux.HandleFunc("/chfs/shares", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			s.handleShareDelete(w, r, userOf(r))
			return
		}
		s.handleShareList(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/trash", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			s.handleTrashList(w, r, userOf(r))
			return
		}
		s.handleTrashAction(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/exist", func(w http.ResponseWriter, r *http.Request) {
		s.handleExist(w, r, userOf(r))
	})
	mux.HandleFunc("/chfs/stat", func(w http.ResponseWriter, r *http.Request) {
		s.handleStat(w, r, userOf(r))
	})
	mux.HandleFunc("/s/", s.handlePublicShare)
	mux.Handle("/webdav/", s.webdavHandler())
	mux.Handle("/webdav", s.webdavHandler())

	handler := s.middleware(mux, log)

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 15 * time.Second}

	go func() {
		fmt.Printf("ModernFS v%s 已启动\n", version)
		fmt.Printf("  监听: %s (HTTPS: %v)\n", addr, s.secure)
		fmt.Printf("  共享根: %s\n", vfs.root)
		for name, target := range vfs.mounts {
			fmt.Printf("  挂载: /%s -> %s\n", name, target)
		}
		fmt.Printf("  配置: %s\n", cfg.File)
		fmt.Printf("  用户: %v\n", cfg.UserOrder)
		var err error
		if s.secure {
			err = srv.ListenAndServeTLS(cfg.SSLCert, cfg.SSLKey)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "服务启动失败:", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	_ = srv.Close()
	fmt.Println("已停止")
}
