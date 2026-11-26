# 配置说明

配置文件为 INI 格式，启动时通过 `-file` 指定，默认查找 `chfs.ini`。

规则：

- 键不存在或值为空时，使用默认值
- 值的语法与对应的命令行参数一致
- 账户段必须放在文件后半部分

## 通用项

### `port`

监听端口。启用 HTTPS 时应为 `443`。

```ini
port=443
```

### `path`

共享根目录，多个目录用 `|` 分隔。

```ini
path=/srv/chfs/public | /srv/chfs/team
```

- 第一个目录作为虚拟根 `/`，其余目录挂载为 `/<目录名>`
- 路径含空格时用引号包裹：`path="C:\My Files\share"`

### `allow`

IP 白名单，留空表示不限制。支持三种写法：

```ini
allow=192.168.1.0/24 | 10.0.* | 203.0.113.7
```

- CIDR：`192.168.1.0/24`
- 前缀通配：`10.0.*`
- 精确 IP：`203.0.113.7`

### `log`

访问日志目录。留空表示禁用。

```ini
log=/var/log/modernfs
```

日志按天切分，文件名 `modernfs-YYYYMMDD.log`，格式：

```
2026-11-24 17:50:01 203.0.113.7 "GET /chfs/files?filepath=/" 200 12ms
```

### `html.title`

网页标题，显示在浏览器标签与页面顶部。

### `html.notice`

顶部公告。纯文本直接写；HTML 需整段用反引号包裹：

```ini
html.notice=`<b>维护通知</b>：本周六 02:00-04:00 停机`
```

### `html.icp` / `html.police`

页脚备案号，可留空。

### `image.preview`

是否生成图片缩略图，默认 `false`。

### `folder.download`

目录打包下载策略：

| 值 | 行为 |
|---|---|
| `enable` | 任意目录均可打包下载（默认） |
| `leaf` | 仅不含子目录的叶子目录可打包 |
| `disable` | 禁止目录打包 |

### `ssl.cert` / `ssl.key`

HTTPS 证书与私钥路径，两者都非空才启用 TLS。

```ini
ssl.cert=/etc/modernfs/tls/fullchain.pem
ssl.key=/etc/modernfs/tls/privkey.pem
```

### `session.timeout`

会话有效期，单位分钟，默认 `30`。

### `file.remove`

删除模式：

| 值 | 行为 |
|---|---|
| `1` | 移到系统回收站（依赖桌面环境，服务器上通常不可用） |
| `2` | 移到程序专属回收站 `.modernfs_trash`，可从 `/chfs/trash` 恢复（**推荐**） |
| `3` | 永久删除 |

专属回收站位于配置目录（即 ini 所在目录）下的 `.modernfs_trash`，索引为 `.modernfs_trash.json`。

## 账户段

```ini
[账号名]
password=密码
rule.default=rwd
rule.none=
rule.r=
rule.w=
rule.d=
```

| 字段 | 说明 |
|---|---|
| `password` | 密码。`guest` 可留空表示免密访问 |
| `rule.default` | 对该账号可见的所有目录的默认权限 |
| `rule.none` | 禁止访问的目录，目录本身及其内容都不可见 |
| `rule.r` | 只读目录 |
| `rule.w` | 可写目录（上传、新建、改名、覆盖） |
| `rule.d` | 完全权限（含删除） |

权限字母：

| 字母 | 含义 |
|---|---|
| `r` | 读取、下载、预览 |
| `w` | 上传、新建、重命名、覆盖写入 |
| `d` | 删除（包含 `r` 与 `w`） |

要点：

- `rule.none` 优先于其他规则
- `rule.r/w/d` 只作用于所列目录的内部，目录本身的权限由 `rule.default` 决定
- 多个目录用 `|` 分隔

### 示例

```ini
[guest]
password=
rule.r=/srv/chfs/public

[alice]
password=alice-pass
rule.default=r
rule.none=/srv/chfs/finance
rule.d=/srv/chfs/alice

[bob]
password=bob-pass
rule.default=
rule.r=/srv/chfs/public
rule.d=/srv/chfs/bob
```

- `guest` 只能读 `/srv/chfs/public`
- `alice` 默认只读，看不到 `finance`，对自己的目录有完全权限
- `bob` 默认无权限，只能读 `public`、管理自己的目录

## 兼容性

配置语法与 CuteHttpFileServer 保持一致，原 chfs 的 ini 通常可直接使用。差异在于：

- 不支持 `file.remove=1` 的系统回收站行为（Linux 服务器无桌面环境）
- 程序专属回收站目录为 `.modernfs_trash`（chfs 为 `.chfs_trashbin`）
