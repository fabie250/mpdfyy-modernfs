# HTTP 接口

服务端提供一套 JSON 接口，路径统一以 `/chfs/` 开头。前端页面就是调用这套接口。

## 认证

登录：

```
POST /chfs/session
Content-Type: application/x-www-form-urlencoded

user=admin&pwd=your-password
```

成功后服务端下发会话 Cookie，后续请求带上即可。

- `guest` 账号密码可为空
- 查询当前会话：`GET /chfs/session`
- 登出：`DELETE /chfs/session`

参数名是 **`pwd`**，不是 `password`。

## 响应格式

成功：

```json
{"status": "ok"}
```

失败：

```json
{"error": "缺少文件路径", "code": 400}
```

常见状态码：

| 码 | 含义 |
|---|---|
| 400 | 参数缺失或非法 |
| 403 | 无权限，或目标为共享根/上级目录 |
| 404 | 目标不存在 |
| 500 | 服务端执行失败 |

## 目录与文件

### `GET /chfs/files`

列出目录内容。

| 参数 | 说明 |
|---|---|
| `filepath` | 虚拟路径，`/` 为根 |

### `GET /chfs/search`

| 参数 | 说明 |
|---|---|
| `str` | 关键词 |

### `POST /chfs/newdir`

| 参数 | 说明 |
|---|---|
| `filepath` | 新目录的完整虚拟路径 |

### `POST /chfs/rename`

| 参数 | 说明 |
|---|---|
| `old` | 原虚拟路径 |
| `new` | 新虚拟路径 |

需要原路径所在目录的删除权限、新路径所在目录的写权限。

### `DELETE /chfs/rmfiles`

| 参数 | 说明 |
|---|---|
| `filepath` | 待删除的虚拟路径 |

行为取决于 `file.remove`：

- `2`（默认）：移入 `.modernfs_trash`，可在 `/chfs/trash` 恢复
- `3`：永久删除

以下请求一律拒绝：

- 空路径或 `/`
- 解析结果落在共享根或其上级目录
- 目标是配置文件或正在运行的二进制

### `GET /chfs/exist`

| 参数 | 说明 |
|---|---|
| `file` | 虚拟路径 |

### `GET /chfs/stat`

| 参数 | 说明 |
|---|---|
| `file` | 虚拟路径 |

返回文件大小、修改时间、类型等信息。

## 上传与下载

### `POST /chfs/upload`

表单上传单文件。

| 参数 | 说明 |
|---|---|
| `folder` | 目标目录虚拟路径 |
| `path` | 可选，相对路径，支持子目录 |
| 文件字段 | 上传的文件内容 |

### `POST /chfs/upload2`

分块/流式上传，参数同 `upload`。

### `GET /chfs/raw`

下载文件。

| 参数 | 说明 |
|---|---|
| `file` | 文件虚拟路径 |

### `GET /chfs/downloaddir`

打包下载目录，路径位于 URL 中。受 `folder.download` 策略约束。

### `GET /chfs/thumb`

图片缩略图。

| 参数 | 说明 |
|---|---|
| `file` | 图片虚拟路径 |
| `w` | 可选，目标宽度像素 |

### `GET /chfs/text`

读取文本内容，用于在线预览/编辑。

| 参数 | 说明 |
|---|---|
| `file` | 文件虚拟路径 |

### `POST /chfs/updatetext`

保存文本。

| 参数 | 说明 |
|---|---|
| `filepath` | 所在目录虚拟路径 |
| `filename` | 文件名 |
| `text` | 新内容 |

### `POST /chfs/newtext`

新建文本文件。

| 参数 | 说明 |
|---|---|
| `filepath` | 所在目录虚拟路径 |
| `title` | 文件名 |
| `text` | 内容 |

## 分享

### `POST /chfs/share`

创建分享链接。

| 参数 | 说明 |
|---|---|
| `filepath` | 目标虚拟路径 |
| `ttl` | 有效期，如 `1h`、`1d`、`7d` |
| `password` | 可选，访问密码 |

返回分享 ID，访问地址为 `/s/<id>`。

### `GET /chfs/share` / `GET /chfs/shares`

列出当前用户的分享。

### `DELETE /chfs/share` / `DELETE /chfs/shares`

删除分享。

### `GET /s/<id>`

公开访问分享内容；设置了密码时附带 `pwd` 参数或通过表单提交。

## 回收站

### `GET /chfs/trash`

列出回收站条目。

### `POST /chfs/trash`

| 参数 | 说明 |
|---|---|
| `id` | 条目 ID |
| `action` | `restore` 还原 / `purge` 彻底删除 / `empty` 清空 |

## 其他

### `GET /chfs/config`

返回站点信息（标题、公告、备案号、当前用户、权限等），前端初始化时调用。

### `GET /chfs/qr`

生成二维码。

| 参数 | 说明 |
|---|---|
| `text` | 要编码的文本或链接 |

### `GET /chfs/shared`

返回公共目录列表。

| 参数 | 说明 |
|---|---|
| `file` | 可选，指定路径 |

## WebDAV

挂载点：

```
/webdav/
```

使用与网页端相同的账号密码，支持 `PROPFIND`、`GET`、`PUT`、`MKCOL`、`MOVE`、`DELETE` 等标准方法。

Windows 挂载示例：

```
\\server\webdav
```

macOS Finder：`Go → Connect to Server → https://host/webdav/`
