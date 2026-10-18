# ai-chat-web 

## 回答来源标识

每条机器人回答的时间戳后显示醒目的来源标签，用户消息不显示：

- **绿色「向量数据库命中」**：本次请求直接使用向量数据库检索命中的答案。
- **蓝色「大模型 API 返回」**：本次请求由大模型接口生成答案，即使该答案随后写入向量数据库，仍显示此来源。
- **黄色「系统敏感词拦截」**：系统拦截提示，不属于向量库答案或模型生成内容。
- 等待响应时显示「来源识别中」；旧记录、旧版服务或未提供来源的响应显示「来源未知」，不推测来源。

来源字段随聊天记录一起保存在本地，刷新页面后保留；重新生成回答时会清除旧来源，并按新响应更新。标签支持深色模式、移动端换行和现有的中英文切换。

来源由 `ai-chat-service` 的实际处理分支写入流式和非流式响应的 `source` 字段（`vector_db`、`llm_api`、`sensitive_filter`），由 `ai-chat-backend` 透传到 `/chat-process` 的每个 JSON 数据块。发布本功能时需同步更新服务端、后端及前端；两个 Go 模块内的聊天协议和生成代码保持一致。

## node 安装
### windows
1. 上[官网](https://nodejs.org/en)下载18.16.0 LTS版本
2. 查看node 是否安装成功
```
node -v
npm -v
```
3. 安装pnpm
```
npm install pnpm  -g
```

### ubuntu
1. 设置 apt 源，设置后可查看/etc/apt/sources.list.d/nodesource.list 文件
```
curl -sL https://deb.nodesource.com/setup_18.x | sudo -E bash -
```
2. 安装nodejs
```
sudo apt-get install -y nodejs
```
3. 验证
```
node -v
npm -v
```
4. 安装pnpm
```
sudo npm install pnpm -g
```

## 编译运行
1. 依赖安装
``` 
pnpm bootstrap
```
2. 本地运行
```
pnpm dev
```
3. 打包发布版本
```
pnpm build-only
```

## 提交代码的规则
```
* commitlint 规则是指在提交代码时要遵循的规范，常见的 commitlint 规则如下：

* type：用于说明 commit 的类型，例如 feat（新功能）、fix（修复 bug）、docs（文档更新）、style（样式修改）、refactor（重构代码）等。

* scope：用于说明 commit 影响的范围，例如组件、模块、页面等。

* subject：用于简短地描述 commit 的内容，建议不超过 50 个字符。

* body：用于详细描述 commit 的改动内容，可以分成多行。

* footer：用于关闭 issue 或者添加相关链接等信息。

* 长度限制：commit message 不应该过长，一般不超过 72 个字符。
```