# 编译阶段
FROM golang:1.25-alpine AS stage0
WORKDIR /src/keywords-filter
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=readonly -o keywords-filter ./filter-server

FROM alpine:3.22 AS stage1
RUN apk add --no-cache ca-certificates
COPY ./grpc_health_probe-linux-amd64 /usr/bin/grpc_health_probe
RUN chmod +x /usr/bin/grpc_health_probe
WORKDIR /app/
COPY ./dev.config.yaml /app/config.yaml
COPY ./dict.txt /app/dict.txt
COPY --from=stage0 /src/keywords-filter/keywords-filter ./
# 指定入口程序
ENTRYPOINT ["./keywords-filter"]
# 指定容器的启动命令或者入口程序的参数
CMD ["--config=config.yaml","--dict=dict.txt"]
