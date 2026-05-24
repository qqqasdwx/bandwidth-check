# bandwidth-check

一个轻量 Docker 探针，用于检查中兴路由器 WAN 口协商速率。程序会登录路由器 Web 后台，读取网口状态，并把结果推送到 Uptime Kuma Push Monitor，由 Kuma 负责告警。

## Uptime Kuma 配置

在 Uptime Kuma 中新建监控：

- Monitor Type：`Push`
- Name：`Router WAN Speed`
- Heartbeat Interval：建议 `90` 或 `120` 秒
- Retries：如果希望异常立即告警，设为 `0`

创建后复制 Kuma 生成的 Push URL，后面作为 `KUMA_PUSH_URL` 传给容器。

## 配置项

常用配置如下：

```text
ROUTER_URL=http://your-router-address
ROUTER_USERNAME=admin
ROUTER_PASSWORD=路由器后台密码
KUMA_PUSH_URL=Uptime Kuma Push URL
WAN_PORT_ALIAS=ETH_WAN
MIN_SPEED_MBPS=1000
CHECK_INTERVAL_SECONDS=60
HTTP_TIMEOUT_SECONDS=10
```

必填项是 `ROUTER_URL`、`ROUTER_PASSWORD` 和 `KUMA_PUSH_URL`。不要把真实密码或 Kuma Push URL 提交到 Git。

## Docker 运行

```bash
docker run -d \
  --name bandwidth-check \
  --restart unless-stopped \
  -e ROUTER_URL="http://your-router-address" \
  -e ROUTER_USERNAME="admin" \
  -e ROUTER_PASSWORD="your-router-password" \
  -e KUMA_PUSH_URL="https://kuma.example.com/api/push/your-push-token" \
  -e WAN_PORT_ALIAS="ETH_WAN" \
  -e MIN_SPEED_MBPS="1000" \
  -e CHECK_INTERVAL_SECONDS="60" \
  -e HTTP_TIMEOUT_SECONDS="10" \
  ghcr.io/qqqasdwx/bandwidth-check:latest
```

只执行一次检查：

```bash
docker run --rm \
  -e ROUTER_URL="http://your-router-address" \
  -e ROUTER_USERNAME="admin" \
  -e ROUTER_PASSWORD="your-router-password" \
  -e KUMA_PUSH_URL="https://kuma.example.com/api/push/your-push-token" \
  -e RUN_ONCE="true" \
  ghcr.io/qqqasdwx/bandwidth-check:latest
```

## Docker Compose 运行

`compose.yml` 已把配置直接写在 `environment` 里。部署前先编辑里面的三个值：

```yaml
ROUTER_URL: "http://your-router-address"
ROUTER_PASSWORD: "your-router-password"
KUMA_PUSH_URL: "https://kuma.example.com/api/push/your-push-token"
```

启动：

```bash
docker compose up -d
```

查看日志：

```bash
docker logs -f bandwidth-check
```

## 本地构建

```bash
docker build -t bandwidth-check .
```

## 镜像发布

GitHub Actions 会构建 `linux/amd64` 和 `linux/arm64` 镜像，并发布到：

```text
ghcr.io/qqqasdwx/bandwidth-check
```

如果首次发布后 GHCR 包仍是私有，需要到 GitHub Packages 页面手动改为 Public。

## 状态判断

程序默认检查 `ETH_WAN`：

- 正常：网口已连接，且协商速率 `>= 1000 Mbps`
- 异常：网口断开、速率未知、速率低于阈值，或路由器读取失败

异常时会向 Kuma 推送 `status=down`；正常时推送 `status=up`。
