# 编译阶段
FROM golang:1.25-alpine AS stage0
WORKDIR /src/mock-openai-api
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=readonly -o mock-openai-api .

FROM alpine:3.22 AS stage1
RUN apk add --no-cache ca-certificates curl
WORKDIR /app/
COPY ./dev.config.yaml /app/config.yaml
COPY --from=stage0 /src/mock-openai-api/mock-openai-api ./
# 指定入口程序
ENTRYPOINT ["./mock-openai-api"]
# 指定容器的启动命令或者入口程序的参数
CMD ["--config=config.yaml"]
