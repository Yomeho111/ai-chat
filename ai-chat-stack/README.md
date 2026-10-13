# 本地 Docker Compose 启动

需要 Docker Engine 28 或更新版本及 Docker Compose v2。旧版 Docker 的回环端口发布可能被同一二层网络中的其他主机访问，不满足本地网页访问限制。应用镜像从仓库源码本地构建，不再访问原来的私有镜像仓库；构建时仍需联网下载官方基础镜像、Go/Python 依赖、tokenizer 词表和向量模型。Redis 使用官方镜像。

## 启动

在仓库根目录执行：

```bash
cd /home/mingfeng/11.4-ai-chat

docker --version
docker compose version

docker compose -f ai-chat-stack/compose.yaml config --quiet

docker compose -f ai-chat-stack/compose.yaml up -d \
  --build \
  --wait \
  --wait-timeout 1800

docker compose -f ai-chat-stack/compose.yaml ps
```

也可以进入本目录，执行 `docker compose up -d --build --wait --wait-timeout 1800`。首次启动需要下载默认向量模型，后续会复用 `model-cache` 卷。

如果前两个版本检查命令提示 `docker: command not found`，需先安装 Docker Engine 和 Docker Compose v2。

浏览器访问 <http://127.0.0.1:7080>。唯一发布到宿主机的端口是 `127.0.0.1:7080:80`，不绑定外部网卡；kvstore、vectordb、Redis、gRPC、tokenizer 和 mock 接口均只在 Compose 网络中使用，不发布宿主机端口。不要额外添加公网反向代理或改为 `7080:80`。

如果 Docker 运行在 SSH 远程主机上，请先在自己的电脑上建立本地隧道，然后打开上述地址：

```bash
ssh -N -L 127.0.0.1:7080:127.0.0.1:7080 用户名@远程主机
```

## 验证聊天流程

```bash
curl -fsS http://127.0.0.1:7080/api/health
curl -N -fsS http://127.0.0.1:7080/api/chat-process \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"你好，介绍一下 Go 语言","options":{}}'
```

当前配置明确使用仓库自带的 `mock-openai-api`：回复从内置文本中随机选择，**不是真实大模型回答**，仅用于验证网页、过滤、分词、聊天流和语义缓存流程。

## 配置与数据

- 容器间通过 `kvstore`、`vectordb`、`vectordb-service`、`redis`、`tokenizer` 等服务名连接，不能使用 `127.0.0.1` 连接其他容器。宿主机直接运行的开发配置使用 `127.0.0.1:50051` 连接 Rag 服务。
- 应用配置和词典来自 [configs](./configs)，存储服务配置复用 [kvstore Compose 配置](../third_party/kvstore/deploy/compose/config)，不需要创建 Docker Swarm config。
- `vectordb-service` 先通过 `Vset` 将关键词向量写入 `vectordb` 并生成 UUID，再通过 `Set` 以同一 UUID 为 key 将回答写入 `kvstore`。查询流程是 `Vsearch` 返回 UUID，再通过 `Get` 读取回答；不再保存完整聊天记录。
- KV、向量、模型和 Redis 数据分别保存在 `kvstore-data`、`vectordb-data`、`model-cache` 和 `redis-data` 命名卷中。
- Redis 的 `123456` 和模型接口的 `local-mock-token` 都是本地演示凭据，不能用于生产环境；它们必须与对应应用配置保持一致。
- [聊天配置](./configs/ai-chat-service.yaml) 使用本地 `vectordb-service:50051`，余弦距离阈值为 `0.01`，等价于原流程要求相似度大于 `0.99`。
- Go 服务包含 amd64 健康检查程序，因此指定 `linux/amd64`；ARM 主机需要 Docker 的 amd64 仿真支持。
- Go 镜像使用 Go 1.25 编译；聊天服务和后端现有的 gRPC 依赖要求最低 Go 1.21，相应模块声明已同步。网页使用后端仓库已有的静态资源，无需另外安装 Node.js 依赖。

如需真实 AI 回复，修改聊天配置中的 `chat.api_key` 和 `chat.base_url`，然后在本目录执行 `docker compose up -d --force-recreate ai-chat-service`。访问宿主机上的模型接口时需额外配置 Docker 的宿主机网关地址，不能直接填写容器内的 `localhost`。可通过 `MODEL_NAME` 环境变量更换向量模型；离线启动前需确保 `model-cache` 已包含模型并设置 `HF_HUB_OFFLINE=1`。

## 日志与停止

查看关键服务日志：

```bash
docker compose -f ai-chat-stack/compose.yaml logs --tail=100 kvstore vectordb vectordb-service ai-chat-service ai-chat-backend
```

停止服务并保留数据卷：

```bash
docker compose -f ai-chat-stack/compose.yaml down
```

停止服务并删除 KV、向量、模型缓存和 Redis 数据卷：

```bash
docker compose -f ai-chat-stack/compose.yaml down -v
```

构建失败时先检查基础镜像、Go 模块和 PyPI 的网络连接；启动失败时查看 `ps`、容器健康检查结果和对应服务日志。
