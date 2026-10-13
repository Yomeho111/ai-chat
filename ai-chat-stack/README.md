
```bash

docker compose -f ai-chat-stack/compose.yaml config --quiet

docker compose -f ai-chat-stack/compose.yaml up -d \
  --build \
  --remove-orphans \
  --wait \
  --wait-timeout 1800

docker compose -f ai-chat-stack/compose.yaml ps
```

<http://127.0.0.1:7080>

聊天上下文缓存在独立的 `context-kvstore` 容器中，使用 `6379` 端口和独立数据卷，不配置密码；它与保存语义缓存回答的 `kvstore` 容器互不共享数据。

语义缓存不再依赖关键词提取服务：向量写入和搜索都直接使用完整问题文本，命中 UUID 后从 `kvstore` 读取对应回答。


```bash
docker compose -f ai-chat-stack/compose.yaml down
```

```bash
docker compose -f ai-chat-stack/compose.yaml down -v
```
