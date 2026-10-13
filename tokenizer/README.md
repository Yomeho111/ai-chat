# tokenizer

## 镜像构建
``` 
docker build -t tokenizer:local .
```

## 服务启动
``` 
docker run --rm --name tokenizer -p 127.0.0.1:3002:3002 tokenizer:local
```

完整的本地聊天服务使用 [Docker Compose](../ai-chat-stack/README.md) 启动，无需私有镜像仓库。