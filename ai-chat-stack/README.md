
```bash

docker compose -f ai-chat-stack/compose.yaml config --quiet

docker compose -f ai-chat-stack/compose.yaml up -d \
  --build \
  --remove-orphans \
  --wait \
  --wait-timeout 1800

docker compose -f ai-chat-stack/compose.yaml ps
```

本机访问：<http://127.0.0.1:7080>

外部访问：`http://<服务器IP>:7080`。需要在主机防火墙和云平台安全组中放行 TCP `7080`；只有 Web 端口会发布到宿主机，kvstore、向量数据库和 gRPC 服务仍仅在 Compose 内部网络可见。

默认监听所有网卡。需要限制监听地址或修改端口时，可在启动命令前设置 `WEB_BIND_ADDRESS` 和 `WEB_PORT`，例如：

```bash
WEB_BIND_ADDRESS=192.0.2.10 WEB_PORT=8080 \
  docker compose -f ai-chat-stack/compose.yaml up -d --remove-orphans
```

当前 Web 接口没有 TLS；开放到公网时应在前面配置带 HTTPS 和访问控制的反向代理。

聊天上下文缓存在独立的 `context-kvstore` 容器中，使用 `6379` 端口和独立数据卷，不配置密码；它与保存语义缓存回答的 `kvstore` 容器互不共享数据。

语义缓存不再依赖关键词提取服务：向量写入和搜索都直接使用完整问题文本，命中 UUID 后从 `kvstore` 读取对应回答。


```bash
docker compose -f ai-chat-stack/compose.yaml down
```

```bash
docker compose -f ai-chat-stack/compose.yaml down -v
```
