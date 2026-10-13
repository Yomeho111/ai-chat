
```bash

docker compose -f ai-chat-stack/compose.yaml config --quiet

docker compose -f ai-chat-stack/compose.yaml up -d \
  --build \
  --wait \
  --wait-timeout 1800

docker compose -f ai-chat-stack/compose.yaml ps
```

<http://127.0.0.1:7080>


```bash
docker compose -f ai-chat-stack/compose.yaml down
```

```bash
docker compose -f ai-chat-stack/compose.yaml down -v
```
