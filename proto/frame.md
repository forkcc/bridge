# 隧道帧格式与协议

## 传输层

- **Client/Edge 与 Bridge**：当前为 **TCP + yamux**（计划支持 QUIC 优先、不可用时降级 TCP）。
- **不加密、不压缩**（方案 A）。

## 帧格式（流内可选）

在单条流上可选用长度前缀帧：

- **4 字节**大端长度 + **负载**。
- 最大帧长 1MB（`pkg/tunnel` 中 `maxFrameSize`）。

## 会话首流

- **Edge 连 Bridge**：首流发送一行 `EDGE <edge_id>\n`，之后 Bridge 将该会话记为该 Edge。
- **Client 连 Bridge**：首流发送一行 `CLIENT <token>\n`，之后 Bridge 接受该 Client 的后续流。

## Client 请求流

每条流对应一次代理连接：

1. Client 写入：`CONNECT <edge_id> <user_id>\n`（user_id 可为 0）。
2. Client 再写入：`CONNECT <host:port>\n`（实际目标地址）。
3. Bridge 将后续数据转发给对应 Edge。
4. Edge 连接目标后回写：`OK\n`。
5. Bridge 将 `OK\n` 及后续数据回传给 Client。
6. 之后双向裸流转发。

## 流与多路复用

- 单条 TCP（或 QUIC）连接上多条流由 **yamux**（或 QUIC 流）区分。
- 每条流独立完成上述 CONNECT 流程后做双向转发。
