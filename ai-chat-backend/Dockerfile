# 编译阶段
FROM golang:1.25-alpine AS stage0
WORKDIR /src/ai-chat-backend
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=readonly -o ai-chat-backend ./cmd

FROM alpine:3.22 AS stage1
RUN apk add --no-cache ca-certificates curl
WORKDIR /app/
COPY ./dev.config.yaml /app/config.yaml
COPY ./www /app/www
COPY --from=stage0 /src/ai-chat-backend/ai-chat-backend ./
# 指定入口程序
ENTRYPOINT ["./ai-chat-backend"]
# 指定容器的启动命令或者入口程序的参数
CMD ["--config=config.yaml"]
