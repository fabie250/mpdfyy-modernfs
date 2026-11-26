package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type GlobalConfig struct {
	Port           int      `json:"port"`
	Paths          []string `json:"paths"`
	Allow          []string `json:"allow"`
	LogDir         string   `json:"logDir"`
	Title          string   `json:"title"`
	Notice         string   `json:"notice"`
	ICP            string   `json:"icp"`
	Police         string   `json:"police"`
	ImagePreview   bool     `json:"imagePreview"`
	FolderDownload string   `json:"folderDownload"`
	SSLCert        string   `json:"sslCert"`
	SSLKey         string   `json:"sslKey"`
	SessionTimeout int      `json:"sessionTimeout"`
	FileRemove     int      `json:"fileRemove"`
}

type UserConfig struct {
	Name     string   `json:"name"`
	Password string   `json:"password"`
	Default  string   `json:"default"`
	None     []string `json:"none"`
	R        []string `json:"r"`
	W        []string `json:"w"`
	D        []string `json:"d"`
}

type AppConfig struct {
	Global GlobalConfig `json:"global"`
	Users  []UserConfig `json:"users"`
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

func cleanLines(v string) []string {
	var out []string
	for _, line := range strings.Split(v, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func defaultConfig() *AppConfig {
	return &AppConfig{
		Global: GlobalConfig{
			Port:           443,
			Paths:          []string{"/srv/chfs/public"},
			Title:          "文件空间",
			ImagePreview:   true,
			FolderDownload: "enable",
			SessionTimeout: 60,
			FileRemove:     3,
		},
		Users: []UserConfig{{Name: "guest", Password: "", Default: "", R: []string{"/srv/chfs/public"}}},
	}
}

func loadConfig(path string) (*AppConfig, error) {
	cfg := defaultConfig()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	defer f.Close()
	cfg.Users = []UserConfig{}

	var cur *UserConfig
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(line[1 : len(line)-1])
			cur = &UserConfig{Name: name}
			cfg.Users = append(cfg.Users, *cur)
			cur = &cfg.Users[len(cfg.Users)-1]
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if cur != nil {
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
		g := &cfg.Global
		switch k {
		case "port":
			if n, err := strconv.Atoi(v); err == nil {
				g.Port = n
			}
		case "path":
			g.Paths = splitPaths(v)
		case "allow":
			g.Allow = splitPaths(v)
		case "log":
			g.LogDir = v
		case "html.title":
			g.Title = v
		case "html.notice":
			g.Notice = v
		case "html.icp":
			g.ICP = v
		case "html.police":
			g.Police = v
		case "image.preview":
			g.ImagePreview = strings.EqualFold(v, "true")
		case "folder.download":
			g.FolderDownload = v
		case "ssl.cert":
			g.SSLCert = v
		case "ssl.key":
			g.SSLKey = v
		case "session.timeout":
			if n, err := strconv.Atoi(v); err == nil {
				g.SessionTimeout = n
			}
		case "file.remove":
			if n, err := strconv.Atoi(v); err == nil {
				g.FileRemove = n
			}
		}
	}
	return cfg, sc.Err()
}

func quotePath(p string) string {
	if strings.ContainsAny(p, " |") {
		return `"` + p + `"`
	}
	return p
}

func joinPaths(ps []string) string {
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, quotePath(p))
		}
	}
	return strings.Join(parts, " | ")
}

func validate(cfg *AppConfig) error {
	if cfg.Global.Allow == nil {
		cfg.Global.Allow = []string{}
	}
	if cfg.Global.Paths == nil {
		cfg.Global.Paths = []string{}
	}
	for i := range cfg.Users {
		u := &cfg.Users[i]
		if u.None == nil {
			u.None = []string{}
		}
		if u.R == nil {
			u.R = []string{}
		}
		if u.W == nil {
			u.W = []string{}
		}
		if u.D == nil {
			u.D = []string{}
		}
	}
	g := cfg.Global
	if g.Port < 1 || g.Port > 65535 {
		return fmt.Errorf("端口必须在 1–65535 之间")
	}
	if len(g.Paths) == 0 {
		return fmt.Errorf("至少填写一个共享目录")
	}
	if g.SessionTimeout < 1 {
		return fmt.Errorf("会话时长必须大于 0 分钟")
	}
	if g.FileRemove < 1 || g.FileRemove > 3 {
		return fmt.Errorf("删除模式只能是 1、2 或 3")
	}
	seen := map[string]bool{}
	for _, u := range cfg.Users {
		if strings.TrimSpace(u.Name) == "" {
			return fmt.Errorf("存在未填写用户名的账号")
		}
		name := strings.TrimSpace(u.Name)
		if seen[name] {
			return fmt.Errorf("账号 %s 重复", name)
		}
		seen[name] = true
		for _, ch := range u.Default {
			if ch != 'r' && ch != 'w' && ch != 'd' {
				return fmt.Errorf("账号 %s 的默认权限只能包含 r、w、d", name)
			}
		}
	}
	return nil
}

func writeConfig(path string, cfg *AppConfig) error {
	if err := validate(cfg); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		backup := path + ".bak-" + time.Now().Format("20060102150405")
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(backup, data, 0644); err != nil {
			return err
		}
	}
	if dir := filepath.Dir(path); dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}
	tmp := path + ".tmp"
	var b strings.Builder
	g := cfg.Global
	b.WriteString("# ModernFS 配置文件，由配置向导生成\n")
	b.WriteString(fmt.Sprintf("port=%d\n", g.Port))
	b.WriteString("path=" + joinPaths(g.Paths) + "\n")
	if len(g.Allow) > 0 {
		b.WriteString("allow=" + joinPaths(g.Allow) + "\n")
	}
	b.WriteString("log=" + g.LogDir + "\n")
	b.WriteString("html.title=" + g.Title + "\n")
	b.WriteString("html.notice=" + g.Notice + "\n")
	if g.ICP != "" {
		b.WriteString("html.icp=" + g.ICP + "\n")
	}
	if g.Police != "" {
		b.WriteString("html.police=" + g.Police + "\n")
	}
	b.WriteString(fmt.Sprintf("image.preview=%t\n", g.ImagePreview))
	if g.FolderDownload != "" {
		b.WriteString("folder.download=" + g.FolderDownload + "\n")
	}
	if g.SSLCert != "" {
		b.WriteString("ssl.cert=" + g.SSLCert + "\n")
	}
	if g.SSLKey != "" {
		b.WriteString("ssl.key=" + g.SSLKey + "\n")
	}
	b.WriteString(fmt.Sprintf("session.timeout=%d\n", g.SessionTimeout))
	b.WriteString(fmt.Sprintf("file.remove=%d\n", g.FileRemove))
	b.WriteString("\n")
	for _, u := range cfg.Users {
		b.WriteString("[" + strings.TrimSpace(u.Name) + "]\n")
		b.WriteString("password=" + u.Password + "\n")
		if u.Default != "" {
			b.WriteString("rule.default=" + u.Default + "\n")
		}
		if len(u.None) > 0 {
			b.WriteString("rule.none=" + joinPaths(u.None) + "\n")
		}
		if len(u.R) > 0 {
			b.WriteString("rule.r=" + joinPaths(u.R) + "\n")
		}
		if len(u.W) > 0 {
			b.WriteString("rule.w=" + joinPaths(u.W) + "\n")
		}
		if len(u.D) > 0 {
			b.WriteString("rule.d=" + joinPaths(u.D) + "\n")
		}
		b.WriteString("\n")
	}
	if err := os.WriteFile(tmp, []byte(b.String()), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func main() {
	cfgPath := flag.String("file", "modernfs.ini", "配置文件路径")
	webPort := flag.Int("web-port", 18099, "本地向导端口")
	noBrowser := flag.Bool("no-browser", false, "不自动打开浏览器")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintln(out, "ModernFS 配置向导")
		fmt.Fprintln(out, "\n用法：")
		fmt.Fprintln(out, "  config-writer.exe [-file 配置文件] [-web-port 端口] [-no-browser]")
		fmt.Fprintln(out, "\n命令行参数：")
		flag.PrintDefaults()
		fmt.Fprintln(out, `
说明：
  向导只监听 127.0.0.1，不会对外暴露；保存前会自动生成 .bak-时间戳 备份。

示例：
  config-writer.exe -file=modernfs.ini
  config-writer.exe -file=modernfs.ini -web-port=18099 -no-browser`)
	}
	flag.Parse()

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "读取配置失败:", err)
		os.Exit(1)
	}

	http.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(cfg)
	})

	http.HandleFunc("/api/save", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
			return
		}
		var next AppConfig
		if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
			http.Error(w, "配置格式无效", http.StatusBadRequest)
			return
		}
		if err := writeConfig(*cfgPath, &next); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cfg = &next
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.WriteString(w, `{"ok":true}`)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(pageHTML))
	})

	addr := "127.0.0.1:" + strconv.Itoa(*webPort)
	url := "http://" + addr + "/"
	fmt.Println("ModernFS 配置向导已启动：", url)
	fmt.Println("配置文件：", *cfgPath)
	if !*noBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			openBrowser(url)
		}()
	}
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintln(os.Stderr, "向导启动失败:", err)
		os.Exit(1)
	}
}

const pageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>ModernFS 配置向导</title>
<style>
:root{color-scheme:light dark;--bg:#f2f3f5;--surface:#fff;--surface2:#f5f6f8;--line:#e5e8ee;--txt:#17181c;--txt2:#666c78;--accent:#2f6df6;--soft:#e9f0ff;--red:#e5484d;--radius:16px}
@media(prefers-color-scheme:dark){:root{--bg:#0a0b0d;--surface:#17181d;--surface2:#1e2026;--line:#2a2d35;--txt:#f2f3f5;--txt2:#a3a9b4;--soft:rgba(91,147,255,.15)}}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--txt);font-family:system-ui,-apple-system,"Microsoft YaHei",sans-serif;padding:28px 18px}
.wrap{max-width:1080px;margin:0 auto}h1{font-size:24px;margin:0 0 6px}p{color:var(--txt2);font-size:13px}.card{background:var(--surface);border:1px solid var(--line);border-radius:var(--radius);padding:20px;margin:16px 0;box-shadow:0 1px 3px rgba(0,0,0,.05)}
.grid{display:grid;grid-template-columns:repeat(4,1fr);gap:14px}label{display:flex;flex-direction:column;gap:6px;font-size:12px;color:var(--txt2)}input,select,textarea{width:100%;background:var(--surface2);border:1px solid var(--line);border-radius:12px;padding:10px 12px;color:var(--txt);font:inherit}textarea{min-height:72px;resize:vertical}.wide{grid-column:span 2}
.user{border:1px solid var(--line);border-radius:14px;padding:16px;margin:12px 0;background:var(--surface2)}.user-grid{display:grid;grid-template-columns:1.2fr 1fr 1fr;gap:12px}.rules{display:grid;grid-template-columns:repeat(3,1fr);gap:12px;margin-top:12px}
button{border:0;border-radius:12px;padding:10px 16px;font:inherit;cursor:pointer}.primary{background:var(--accent);color:#fff}.ghost{background:var(--surface2);color:var(--txt2);border:1px solid var(--line)}.danger{background:rgba(229,72,77,.12);color:var(--red)}.actions{display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-top:16px}.msg{font-size:13px;margin-left:auto}.ok{color:#12a97a}.err{color:var(--red)}
@media(max-width:820px){.grid,.user-grid,.rules{grid-template-columns:1fr}.wide{grid-column:auto}}
</style>
</head>
<body>
<div class="wrap">
<h1>ModernFS 配置向导</h1>
<p>填写后保存到配置文件；保存前会自动生成带时间戳的备份。路径支持多个，每行一个。</p>
<div class="card">
<div class="grid">
<label>端口<input id="port" type="number" min="1" max="65535"></label>
<label>会话时长（分钟）<input id="sessionTimeout" type="number" min="1"></label>
<label>删除模式<select id="fileRemove"><option value="1">1 移入系统回收站</option><option value="2">2 移入专属回收站</option><option value="3">3 永久删除</option></select></label>
<label>目录下载<select id="folderDownload"><option value="enable">不限制</option><option value="leaf">仅叶子目录</option><option value="disable">禁用</option></select></label>
<label class="wide">网页标题<input id="title"></label>
<label class="wide">公告文字<textarea id="notice"></textarea></label>
<label>ICP 备案号<input id="icp" placeholder="例如 闽ICP备xxxx号"></label>
<label>公安备案号<input id="police" placeholder="例如 闽公网安备xxxx号"></label>
<label>日志目录<input id="logDir"></label>
<label>IP 白名单（每行一个）<textarea id="allow"></textarea></label>
<label class="wide">共享目录（每行一个）<textarea id="paths"></textarea></label>
<label>HTTPS 证书<input id="sslCert"></label>
<label>HTTPS 私钥<input id="sslKey"></label>
</div>
</div>
<div class="card">
<div style="display:flex;align-items:center;justify-content:space-between"><h2 style="margin:0;font-size:17px">账号与权限</h2><button class="ghost" id="addUser" type="button">新增账号</button></div>
<div id="users"></div>
</div>
<div class="actions">
<button class="primary" id="save" type="button">保存配置</button>
<span class="msg" id="msg"></span>
</div>
</div>
<script>
let cfg=null;
const $=id=>document.getElementById(id);
const lines=a=>(a||[]).join('\n');
function ruleText(arr){return (arr||[]).join('\n')}
function esc(s){return String(s||'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
function renderUsers(){
  const box=$('users');box.innerHTML='';
  cfg.users.forEach((u,idx)=>{
    const el=document.createElement('div');el.className='user';
    el.innerHTML='<div class="user-grid">' +
      '<label>用户名<input data-k="name" value="'+esc(u.name)+'"></label>' +
      '<label>密码<input data-k="password" type="password" value="'+esc(u.password)+'"></label>' +
      '<label>默认权限<select data-k="default"><option value="">无权限</option><option value="r">只读 R</option><option value="rw">读写 RW</option><option value="rwd">完全权限 RWD</option></select></label>' +
    '</div>' +
    '<div class="rules">' +
      '<label>只读目录（每行一个）<textarea data-k="r">'+esc(ruleText(u.r))+'</textarea></label>' +
      '<label>可写目录（每行一个）<textarea data-k="w">'+esc(ruleText(u.w))+'</textarea></label>' +
      '<label>最高权限目录（每行一个）<textarea data-k="d">'+esc(ruleText(u.d))+'</textarea></label>' +
    '</div>' +
    '<div class="actions"><button class="danger" type="button" data-del>删除账号</button></div>';
    el.querySelector('[data-del]').onclick=()=>{cfg.users.splice(idx,1);renderUsers()};
    el.querySelectorAll('input,select,textarea').forEach(inp=>{
      inp.oninput=()=>{
        const k=inp.dataset.k;
        if(k==='r'||k==='w'||k==='d'){u[k]=cleanLines(inp.value)}else{u[k]=inp.value}
      };
    });
    el.querySelector('[data-k="default"]').value=u.default||'';
    box.appendChild(el);
  });
}
function cleanLines(v){return v.split('\n').map(s=>s.trim()).filter(Boolean)}
function fillGlobal(){
  const g=cfg.global;
  $('port').value=g.port;$('sessionTimeout').value=g.sessionTimeout;$('fileRemove').value=g.fileRemove;
  $('folderDownload').value=g.folderDownload||'enable';$('title').value=g.title;$('notice').value=g.notice;
  $('icp').value=g.icp||'';$('police').value=g.police||'';$('logDir').value=g.logDir||'';
  $('allow').value=lines(g.allow);$('paths').value=lines(g.paths);$('sslCert').value=g.sslCert||'';$('sslKey').value=g.sslKey||'';
}
function readGlobal(){
  cfg.global.port=Number($('port').value);cfg.global.sessionTimeout=Number($('sessionTimeout').value);
  cfg.global.fileRemove=Number($('fileRemove').value);cfg.global.folderDownload=$('folderDownload').value;
  cfg.global.title=$('title').value.trim();cfg.global.notice=$('notice').value.trim();cfg.global.icp=$('icp').value.trim();
  cfg.global.police=$('police').value.trim();cfg.global.logDir=$('logDir').value.trim();cfg.global.allow=cleanLines($('allow').value);
  cfg.global.paths=cleanLines($('paths').value);cfg.global.sslCert=$('sslCert').value.trim();cfg.global.sslKey=$('sslKey').value.trim();
}
async function init(){
  cfg=await (await fetch('/api/config')).json();
  fillGlobal();renderUsers();
}
$('addUser').onclick=()=>{cfg.users.push({name:'',password:'',default:'',none:[],r:[],w:[],d:[]});renderUsers()};
$('save').onclick=async()=>{
  readGlobal();
  $('msg').className='msg';$('msg').textContent='正在保存…';
  const res=await fetch('/api/save',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(cfg)});
  if(res.ok){$('msg').className='msg ok';$('msg').textContent='已保存，原配置已自动备份'}
  else{$('msg').className='msg err';$('msg').textContent=await res.text()}
};
init();
</script>
</body>
</html>`
