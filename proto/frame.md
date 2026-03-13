# 隧道帧格式与协议

## 数据流模式（必须遵守）

流量**必须**经 Bridge 中转，不允许 Client 与 Edge 直连：

- **请求方向**：`Client → Bridge → Edge → 目标`
- **响应方向**：`目标 → Edge → Bridge → Client`

即：**client → bridge → edge**，**edge → bridge → client**。Bridge 在中间做双向转发，Edge 与 Client 之间没有独立连接。

## 传输层

- **Client/Edge 与 Bridge**：当前为 **纯 TCP + yamux**。
- **不加密、不压缩**。内网部署或外层已有加密（VPN / WireGuard 等）时可直接使用。

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
