# ModernFS

一个用 Go 写的轻量文件共享服务，接口与配置文件格式兼容 [CuteHttpFileServer (chfs)](https://github.com/docblue/chfs)，可直接替换原版使用。

前端为单页界面，资源通过 `go:embed` 内嵌进二进制，**单文件即可运行**，无需额外依赖。

```
ModernFS v1.0.0
  监听: :8080 (HTTPS: false)
  共享根: /srv/chfs/public
  挂载: /team -> /srv/chfs/team
  配置文件: modernfs.ini
```

## 特性

- **单文件部署**：前端资源内嵌，一个二进制 + 一个 ini 即可运行
- **兼容 chfs 配置**：沿用 `port`、`path`、`[账号]`、`rule.*` 等原有写法，迁移成本低
- **多目录挂载**：`path` 用 `|` 分隔多个目录，自动映射为虚拟根下的子目录
- **细粒度权限**：每个账号可对任意子目录单独设置 `r` / `w` / `d` 权限
- **程序专属回收站**：删除进入 `.modernfs_trash`，可从 `/chfs/trash` 还原或彻底清除
- **在线预览**：图片缩略图、文本编辑、Office/PDF 预览（前端渲染）
- **分享链接**：生成带有效期和可选密码的公开链接（`/s/<id>`）
- **WebDAV**：挂载为网络驱动器使用
- **IP 白名单**：支持 CIDR 与前缀通配
- **访问日志**：按天切分，记录 IP、方法、路径、状态码与耗时
- **HTTPS**：配置证书与私钥后自动启用

## 快速开始

### 1. 准备配置

```bash
cp examples/modernfs.example.ini modernfs.ini
```

至少修改 `path` 为真实的共享目录，并在 `[admin]` 段设置密码。

### 2. 运行

```bash
./modernfs-linux-amd64 -file=modernfs.ini
```

也可以覆盖端口：

```bash
./modernfs-linux-amd64 -file=modernfs.ini -port=8080
```

打开 `http://127.0.0.1:8080` 即可访问。

### 3. 注册为系统服务（Linux）

```bash
sudo install -m 755 modernfs-linux-amd64 /usr/local/bin/modernfs
sudo mkdir -p /etc/modernfs
sudo cp modernfs.ini /etc/modernfs/modernfs.ini
sudo cp scripts/modernfs.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now modernfs
```

## 配置说明

完整字段见 [`docs/configuration.md`](docs/configuration.md)，常用项：

| 配置项 | 说明 |
|---|---|
| `port` | 监听端口 |
| `path` | 共享根目录，多个用 `\|` 分隔 |
| `allow` | IP 白名单，留空不限制 |
| `log` | 访问日志目录，留空禁用 |
| `html.title` | 网页标题 |
| `html.notice` | 顶部公告，支持 HTML |
| `html.icp` / `html.police` | 备案号，显示在页脚 |
| `image.preview` | 是否生成图片缩略图 |
| `folder.download` | 目录打包下载：`enable` / `leaf` / `disable` |
| `ssl.cert` / `ssl.key` | HTTPS 证书与私钥 |
| `session.timeout` | 会话有效期（分钟） |
| `file.remove` | 删除模式：`1` 系统回收站，`2` 程序回收站，`3` 永久删除 |

### 账号与权限

```ini
[fabie]
password=your-password
rule.default=rwd
rule.none=/srv/chfs/private
rule.r=/srv/chfs/public
rule.w=/srv/chfs/team
rule.d=/srv/chfs/fabie
```

- `guest` 为访客账号，`password` 可留空
- `r` 只读、`w` 可写、`d` 完全权限（含删除）
- `rule.none` 的目标目录及其内容均不可访问
- `rule.r/w/d` 只作用于所列目录**内部**，目录本身权限由 `rule.default` 决定
- 多个目录用 `|` 分隔

## 接口

服务端提供一套 `/chfs/*` JSON 接口，前端即调用这套接口，也可自行集成。详见 [`docs/api.md`](docs/api.md)。

主要端点：

| 路径 | 方法 | 用途 |
|---|---|---|
| `/chfs/session` | POST | 登录 / 登出 |
| `/chfs/config` | GET | 站点配置与当前用户 |
| `/chfs/files` | GET | 列出目录 |
| `/chfs/search` | GET | 搜索 |
| `/chfs/upload` | POST | 表单上传 |
| `/chfs/upload2` | POST | 分块上传 |
| `/chfs/newdir` | POST | 新建目录 |
| `/chfs/newtext` | POST | 新建文本 |
| `/chfs/updatetext` | POST | 保存文本 |
| `/chfs/rename` | POST | 重命名 |
| `/chfs/rmfiles` | DELETE | 删除 |
| `/chfs/raw` | GET | 下载文件 |
| `/chfs/downloaddir` | GET | 打包下载目录 |
| `/chfs/thumb` | GET | 图片缩略图 |
| `/chfs/share` | GET/POST/DELETE | 分享管理 |
| `/chfs/trash` | GET/POST | 回收站 |
| `/s/<id>` | GET | 公开分享访问 |
| `/webdav/` | WebDAV | 网络驱动器 |

## 构建

要求 **Go 1.26+**。

```bash
# Linux amd64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o modernfs-linux-amd64 .

# Windows
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o modernfs-windows-amd64.exe .
```

或使用脚本：

```bash
./scripts/build.sh          # Linux/macOS
pwsh ./scripts/build.ps1    # Windows
```

## 配置向导

`cmd/config-writer` 是一个 Windows 桌面配置工具，浏览器界面编辑 ini，保存时自动生成带时间戳的备份。

```bash
cd cmd/config-writer
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o config-writer.exe .
```

用法见 [`docs/config-writer.md`](docs/config-writer.md)。

## 安全说明

本项目在 1.0.0 中修复了几个高危缺陷，升级前请务必了解：

1. **删除接口路径解析**：`DELETE /chfs/rmfiles` 在 `application/x-www-form-urlencoded` 下曾无法解析 body，`filepath` 为空时会被当成共享根，导致整个共享目录被删除。
   现在：空路径返回 `400`，根路径返回 `400`，解析结果落在共享根或上级目录返回 `403`。
2. **重命名越权**：`POST /chfs/rename` 同样禁止空路径、根目录和上级目录。
3. **WebDAV 越权**：`RemoveAll` 与 `Rename` 增加根目录与上级目录保护。
4. **关键文件保护**：配置文件与运行中的二进制禁止通过接口删除。

部署时建议：

- 不要让服务以 root 运行，使用专用用户并限制共享目录权限
- `file.remove` 保持为 `2`，保留可恢复窗口
- 对外暴露时配置 HTTPS，并设置 `allow` 白名单（如果访问来源固定）
- 定期备份配置文件（其中含明文密码，注意存放权限）

## 目录结构

```
.
├── main.go             入口、路由、静态资源、访问日志
├── api.go              /chfs/* 接口实现
├── auth.go             账号、会话与权限判定
├── config.go           ini 解析
├── store.go            分享与回收站持久化
├── vfs.go              虚拟路径映射与越权防护
├── webdav.go           WebDAV 适配
├── assets/             前端资源（内嵌）
├── cmd/config-writer/  配置向导（Windows）
├── docs/               文档
├── examples/           配置示例
└── scripts/            构建与部署脚本
```

## 第三方组件

前端 `assets/vendor/` 下的库均为原样引入，遵循各自许可：

- [JSZip](https://github.com/Stuk/jszip) — MIT
- [SheetJS (xlsx)](https://sheetjs.com/) — Apache-2.0
- [docx-preview](https://github.com/VolodymyrBaydalka/docxjs) — Apache-2.0

后端依赖：

- [skip2/go-qrcode](https://github.com/skip2/go-qrcode) — MIT

## License

Copyright (C) 2026 mpdfyy

本项目以 **GNU General Public License v3.0**（GPL-3.0）发布，许可证全文见 [LICENSE](LICENSE)。

这意味着：你可以自由使用、修改、分发本项目（含商业用途），但对外分发时必须同样以 GPL-3.0 提供完整源代码，并保留版权声明与许可证全文。如需闭源再分发等其他授权方式，须另行取得版权持有人书面许可。
