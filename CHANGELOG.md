# Changelog

本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [1.0.0] - 2026-11-20

首个稳定版本。

### 新增

- 兼容 chfs 的 ini 配置与账号权限语法（`rule.default` / `rule.none` / `rule.r` / `rule.w` / `rule.d`）
- 多目录挂载，`path` 支持 `|` 分隔
- 程序专属回收站（`file.remove=2`），支持 `/chfs/trash` 列表、还原、彻底删除
- 分享链接，支持有效期与访问密码（`/s/<id>`）
- WebDAV 挂载（`/webdav/`）
- 图片缩略图、文本在线编辑、Office/PDF 前端预览
- IP 白名单，支持 CIDR 与前缀通配
- 按天切分的访问日志
- HTTPS 支持
- 配置向导 `cmd/config-writer`（Windows，浏览器界面，保存前自动备份）

### 修复

- **高危**：`DELETE /chfs/rmfiles` 在 urlencoded body 下路径解析为空时，会误删整个共享根。现禁止空路径、根路径，并拦截落在共享根或上级目录的请求。
- **高危**：`POST /chfs/rename` 允许空路径与根目录操作，现同步加固。
- **高危**：WebDAV `RemoveAll` / `Rename` 缺少根目录与上级目录保护，现同步加固。
- 配置文件和运行中的二进制不再允许通过接口删除。

### 变更

- 默认删除模式调整为专属回收站，而不是直接删除。
- 前端资源改为 `go:embed` 内嵌，不再依赖外部文件目录。
- 许可证由 MIT 变更为 GNU General Public License v3.0（GPL-3.0）。
