
```bash

docker compose -f ai-chat-stack/compose.yaml config --quiet

docker compose -f ai-chat-stack/compose.yaml up -d \
  --build \
  --remove-orphans \
  --wait \
  --wait-timeout 1800

docker compose -f ai-chat-stack/compose.yaml up -f ai-chat-stack/compose.openai.yaml -d \
  --build \
  --remove-orphans \
  --wait \
  --wait-timeout 1800

docker compose -f ai-chat-stack/compose.yaml ps
```

CPU 配额按 Docker 可用 2 核配置：`kvstore`、`vectordb` 和 `vectordb-service` 各自最多使用 2 核，`context-kvstore` 最多使用 1 核。`cpus` 是单个容器的使用上限，不是独占预留，各容器共享宿主机 CPU；这些上限无需相加后小于宿主机核数。调整配额时，单个容器的值不能超过 Docker 可用的 CPU 核数，否则会报 `range of CPUs is from 0.01 to 2.00` 一类错误。

仅修改 CPU 配额时无需重新构建镜像，可重新执行上述启动命令并省略 `--build`，由 Compose 重建受影响的容器；无需先执行 `down` 或删除数据卷。

Compose 的后端镜像使用 [Dockerfile.backend](./Dockerfile.backend)，在同一次构建中编译 `ai-chat-web`，再将生成的 `dist` 拷贝到后端的 `www`。不要依赖 `ai-chat-backend/www` 中的旧打包文件。修改前端功能后需使用 `--build` 更新部署；无需手工拷贝静态文件。浏览器已打开旧版本时，重新加载页面以使用新的资源。

默认栈连接的是 `mock-openai-api`，不是收费模型接口。页面的「大模型 API 返回」表示本次请求走了模型接口分支；Token 消耗是 tokenizer 的模拟估算，不表示实际付费账单。

### 切换 OpenAI 官方接口

保留 `compose.yaml` 中的默认配置。官方模式先通过环境变量提供 `OPENAI_API_KEY`，然后同时加载 [compose.openai.yaml](./compose.openai.yaml)：

```bash
docker compose -f ai-chat-stack/compose.yaml \
  -f ai-chat-stack/compose.openai.yaml up -d \
  --build --remove-orphans --wait --wait-timeout 1800
```

覆盖文件会设置 `AI_PROVIDER=openai`，并从 Compose 中移除 `mock-openai-api` 及其健康检查依赖：不构建、不启动 mock；`--remove-orphans` 也会删除之前运行的 mock 容器，不删除数据卷。Compose 不能仅根据容器环境变量条件启停另一个服务，因此不要只修改 `AI_PROVIDER`。未提供密钥时 Compose 直接报错，不回退到 mock；不要提交真实密钥。

恢复 mock 模式时只加载默认文件：

```bash
AI_PROVIDER=mock docker compose -f ai-chat-stack/compose.yaml up -d \
  --remove-orphans --wait --wait-timeout 1800
```

服务通过 HTTPS 请求 `https://api.openai.com/v1/chat/completions`，固定使用 `gpt-6-luna`，支持普通和 SSE 流式回答；不安装或运行本地模型，也不新增依赖。

官方请求使用 `max_completion_tokens`、`reasoning_effort: none`，不发送旧模型的采样和惩罚参数，也不接受后端覆盖官方模型。两种模式不复用彼此的语义缓存回答和聊天上下文，无需删除数据卷。切换后建议新建会话，以免浏览器继续显示旧回答。

GPT-6 Luna 的 Token 统计沿用现有 tokenizer，以 `cl100k_base` 近似估算，仅用于页面统计和本地长度预算；不是官方精确用量或账单，请以 OpenAI 平台为准。

模型接口返回时，消耗量按「完整请求 Token + 回答 Token」估算，节省量为 0。完整请求包含系统提示词、当前问题、按上下文开关及长度预算实际选入的历史消息，以及消息开销。答案与这次生成的总消耗量一起写入带版本的语义缓存；后续命中该答案时，消耗量为 0，节省量直接使用缓存记录的原始生成消耗量，不再按第二次请求新增的历史重新估算。因此第一次生成消耗 89 Token 时，再次命中同一条缓存就节省 89 Token。缓存命中仍校验当前请求长度，超限或请求 tokenizer 失败时明确返回错误。

旧版语义缓存仅保存答案，没有原始消耗量；遇到这类记录时服务会记录原因并重新调用模型生成一次，写入新格式，不猜测旧值，也不需要清空数据卷。浏览器中已有聊天记录保留原统计，不会自动重新计算；请用新请求验证新口径。

回答在接收期间持续显示已收到的文字与闪烁光标，不等待 Token 统计或整段回答完成后才显示。修改后需使用 `--build` 重新构建并更新 `ai-chat-service` 和 `ai-chat-backend`（包含前端），并重新加载浏览器页面。

mock 模型的流式响应使用标准 SSE：每个事件以空行终止，最终先发送带 `finish_reason: stop` 的事件，再发送 `[DONE]`。结束事件缺失会导致回答无法完成、统计无法返回、语义缓存无法写入；健康检查通过并不代表聊天流式链路正常。

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
