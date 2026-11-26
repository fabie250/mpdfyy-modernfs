# 配置向导

`cmd/config-writer` 是一个 Windows 桌面工具，用浏览器界面编辑 `modernfs.ini`，避免手写配置出错。

## 编译

```bash
cd cmd/config-writer
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o config-writer.exe .
```

产物是单文件，不需要安装运行时。

## 使用

1. 把 `config-writer.exe` 和 `modernfs.ini` 放在同一目录
2. 双击运行；浏览器没有自动打开时，访问终端窗口里显示的 `127.0.0.1` 地址
3. 在页面里修改端口、共享目录、标题、公告、备案号、账号和权限规则
4. 点击「保存配置」，保存前会在同目录生成 `modernfs.ini.bak-时间戳` 备份
5. 重启 ModernFS 服务后新配置生效

## 命令行参数

```bash
config-writer.exe -file=modernfs.ini
config-writer.exe -file=modernfs.ini -web-port=18099
config-writer.exe -help
```

| 参数 | 说明 |
|---|---|
| `-file` | 要编辑的配置文件，默认 `modernfs.ini` |
| `-web-port` | 配置页面的本地端口，默认自动选择空闲端口 |
| `-help` | 显示帮助 |

## 权限规则

界面里的权限与 ini 中的 `rule.*` 一一对应：

- **R**：只读，可查看和下载
- **W**：可上传、新建、改名和覆盖写入
- **D**：最高权限，包含 R、W 和删除
- `rule.default` 是未单独指定目录时的默认权限，留空表示默认无权限

## 注意

- 向导只负责写配置文件，不会启动或停止服务
- 配置文件含明文密码，向导生成的备份同样含密码，注意存放位置
- 修改 `port` 或 `ssl.*` 后必须重启服务才生效
