# 部署

以 Linux + systemd 为例。

## 1. 创建运行用户

不要以 root 运行。

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin modernfs
```

## 2. 目录规划

```bash
sudo mkdir -p /opt/modernfs /etc/modernfs /var/log/modernfs
sudo cp modernfs-linux-amd64 /opt/modernfs/modernfs
sudo cp modernfs.ini /etc/modernfs/modernfs.ini
sudo chmod 750 /etc/modernfs /etc/modernfs/modernfs.ini
sudo chown -R modernfs:modernfs /opt/modernfs /var/log/modernfs
```

配置文件含明文密码，权限建议 `750` 或更严格。

## 3. 安装 systemd 单元

```bash
sudo cp scripts/modernfs.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now modernfs
sudo systemctl status modernfs --no-pager
```

启动日志：

```bash
journalctl -u modernfs -f
```

## 4. HTTPS

### 使用已有证书

```ini
port=443
ssl.cert=/etc/modernfs/tls/fullchain.pem
ssl.key=/etc/modernfs/tls/privkey.pem
```

私钥权限设为 `600`，属主为运行用户。

### 使用 Let's Encrypt

```bash
sudo apt install certbot
sudo certbot certonly --standalone -d example.com
```

证书位于 `/etc/letsencrypt/live/example.com/`。

续期后需要重启服务：

```bash
sudo certbot renew --deploy-hook "systemctl restart modernfs"
```

> 注意：续期后若不重启，进程仍持有旧证书。

## 5. 端口与防火墙

```bash
sudo ufw allow 443/tcp
```

若前面有 Nginx 反向代理，服务可只监听 `127.0.0.1:8080`，但本项目监听地址固定为 `:port`，需要靠防火墙限制外部访问。

## 6. Nginx 反向代理（可选）

```nginx
server {
    listen 443 ssl http2;
    server_name example.com;

    ssl_certificate     /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;

    client_max_body_size 0;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_request_buffering off;
    }
}
```

`client_max_body_size 0` 关闭上传大小限制，`proxy_request_buffering off` 避免大文件先落临时盘。服务读取 `X-Real-IP` 作为客户端地址，日志与 IP 白名单都依赖它。

## 7. 日志轮转

访问日志按天切分，但不会自动清理。建议配置 logrotate：

```
/var/log/modernfs/*.log {
    daily
    rotate 14
    compress
    missingok
    notifempty
    copytruncate
}
```

## 8. 备份

需要备份的只有：

- `/etc/modernfs/modernfs.ini`（含账号密码）
- 共享目录中的业务数据（若需要）

`.modernfs_trash` 与 `.modernfs_trash.json` 位于配置文件同目录，用于回收站，可按需保留。

## 9. 升级

```bash
sudo systemctl stop modernfs
sudo cp /opt/modernfs/modernfs /opt/modernfs/modernfs.bak-$(date +%Y%m%d%H%M%S)
sudo install -m 755 modernfs-linux-amd64 /opt/modernfs/modernfs
sudo systemctl start modernfs
curl -k -I https://127.0.0.1/
```

保留上一版二进制，便于回滚。
