# proxy-bridge 全场景分析

> 架构：`Client ←1条连接→ Bridge ←1条连接→ Edge → Target`
>
> 核心约束：Client 和 Edge 各自只与 Bridge 保持 **1 条连接**，通过多路复用所有流量。
>
> 传输层策略：**QUIC 优先，TCP+yamux 降级**。QUIC 不可用时自动回退到 TCP+yamux。

---

## 1. 正常流程

| # | 场景 | 当前状态 | 备注 |
|---|------|---------|------|
| 1.1 | Client 启动 → auth → 获取 edge 列表 → 建 yamux 隧道 → SOCKS5 就绪 | ✅ 已实现 | |
| 1.2 | Edge 启动 → 注册 apiHub → 建 yamux 隧道到 Bridge → 等待 stream | ✅ 已实现 | |
| 1.3 | Bridge 启动 → 注册 apiHub → 监听隧道端口 + HTTP 代理端口 | ✅ 已实现 | |
| 1.4 | SOCKS5 请求 → Client 开 stream → Bridge 转发到 Edge stream → Edge 拨号目标 → 双向转发 | ✅ 已实现 | |
| 1.5 | SOCKS5 CONNECT 域名解析（atyp=3 域名由 Edge 端解析） | ✅ 已实现 | Edge `net.Dial` 自动解析 |
| 1.6 | SOCKS5 IPv4 地址直连（atyp=1） | ✅ 已实现 | |
| 1.7 | SOCKS5 IPv6 地址直连（atyp=4） | ✅ 已实现 | 需 Edge 所在网络支持 IPv6 |
| 1.8 | 请求结束 → stream 关闭 → 流量上报 → traffic_stats 入库 | ✅ 已实现 | |

## 2. 连接生命周期

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 2.1 | Client yamux session 正常存活，多个 SOCKS5 请求复用同一 session | ✅ 已实现 | `ensureTunnel` 缓存 session |
| 2.2 | Edge yamux session 长期存活，Bridge 持续向其 Open stream | ✅ 已实现 | `edgeConns[edgeID]` 保持 |
| 2.3 | Client 优雅退出 → session 关闭 → Bridge 感知 → 心跳停止 → 绑定清理 | ⚠️ 部分 | Client 无 graceful shutdown 信号处理 |
| 2.4 | Edge 优雅退出 → session 关闭 → Bridge 感知 → 清理 edgeConns | ⚠️ 部分 | Edge 无 graceful shutdown，edgeConns 无主动清理 |
| 2.5 | yamux keepalive 维持空闲连接活跃 | ✅ 已实现 | yamux 默认 30s keepalive |
| 2.6 | 单条 stream 双向 copy 结束后正确关闭两端 | ✅ 已实现 | 刚修复 done channel 同步 |

## 3. 组件故障与恢复

### 3.1 Bridge 崩溃

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 3.1.1 | Bridge 崩溃 → Edge 隧道断开 → Edge 自动重连（5s 间隔） | ✅ 已实现 | `for { runTunnel(); sleep 5s }` |
| 3.1.2 | Bridge 崩溃 → Client 隧道断开 → 下次请求 `clearTunnel` + 重建 | ✅ 已实现 | `sess.Open` 失败触发清理 |
| 3.1.3 | Bridge 崩溃 → 正在转发的请求全部中断 | ⚠️ 无法避免 | 用户看到连接重置 |
| 3.1.4 | Bridge 崩溃 → 流量数据丢失（未来得及上报的） | ❌ 未处理 | 可考虑本地缓存待上报数据 |
| 3.1.5 | Bridge 重启 → Edge 重连后 edgeID 恢复 → Client 重连后继续工作 | ✅ 已实现 | |

### 3.2 Edge 崩溃

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 3.2.1 | Edge 崩溃 → Bridge `edgeSession.Open()` 失败 → Client stream 返回错误 | ✅ 已实现 | |
| 3.2.2 | Edge 崩溃 → Bridge `edgeConns` 中残留死 session | ❌ 未处理 | 无清理机制；Open 失败后应删除 |
| 3.2.3 | Edge 崩溃 → 绑定该 Edge 的 Client 无法转发 → Client 需要重新选 Edge | ❌ 未处理 | Client 只 clearTunnel 但不换 Edge |
| 3.2.4 | Edge 崩溃并重启 → 重新注册 → Bridge 收到新 session 替换旧的 | ✅ 已实现 | `old.Close()` 在 handleSession |
| 3.2.5 | Edge 崩溃 → 正在转发的请求中断 → 流量部分丢失 | ⚠️ 部分丢失 | done channel 保证统计到中断点为止 |
| 3.2.6 | Edge 进程被 Android OOM Killer 杀死 | ❌ 未处理 | 需要 Watchdog 或 Android Service 保活 |

### 3.3 Client 崩溃

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 3.3.1 | Client 崩溃 → Bridge session 关闭 → handleClientSession 退出 | ✅ 已实现 | |
| 3.3.2 | Client 崩溃 → 绑定未清理 → Edge 保持 busy → 心跳超时后清理 | ✅ 已实现 | binding_cleanup 每分钟运行 |
| 3.3.3 | Client 崩溃重启 → 重新 auth + ensureTunnel → 重新绑定 | ✅ 已实现 | handleEdgesList 返回已绑定的 edge |

### 3.4 apiHub 崩溃

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 3.4.1 | apiHub 崩溃 → 已建立的隧道不受影响（纯 TCP 转发） | ✅ 已实现 | 隧道不依赖 apiHub |
| 3.4.2 | apiHub 崩溃 → 新 Client 无法 auth/获取 edge 列表 | ⚠️ 预期行为 | Client 报错但不退出 |
| 3.4.3 | apiHub 崩溃 → 心跳失败 → 绑定清理暂停 | ⚠️ 预期行为 | apiHub 恢复后自动继续 |
| 3.4.4 | apiHub 崩溃 → 流量上报失败 → 计费数据丢失 | ❌ 未处理 | 无重试或本地缓存 |
| 3.4.5 | apiHub 崩溃 → Edge 注册/心跳失败但隧道正常 | ✅ 已实现 | Edge 心跳失败只打日志 |

### 3.5 数据库崩溃

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 3.5.1 | PostgreSQL 崩溃 → apiHub 所有 DB 操作失败 → HTTP 500 | ⚠️ 部分 | GORM 会返回错误 |
| 3.5.2 | PostgreSQL 恢复 → GORM 自动重连 | ✅ 已实现 | GORM 内置连接池 |

## 4. 网络故障

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 4.1 | Client→Bridge 网络断开 → yamux session 超时 → Client clearTunnel | ⚠️ 延迟感知 | yamux keepalive 30s 超时 |
| 4.2 | Bridge→Edge 网络断开 → yamux session 超时 → Edge 重连循环 | ⚠️ 延迟感知 | 最多 30s+5s 才能重连 |
| 4.3 | Edge→Target 连接超时（目标不可达） | ✅ 已实现 | `net.Dial` 使用系统默认超时 |
| 4.4 | Edge→Target DNS 解析失败 | ✅ 已实现 | `net.Dial` 返回错误，Edge 关闭 stream |
| 4.5 | 网络抖动导致短暂中断 → yamux 自动恢复（如果未超时） | ✅ 已实现 | TCP 层面重传 |
| 4.6 | 高延迟链路（> 500ms）→ yamux keepalive 可能误判 | ❌ 未处理 | keepalive timeout 可能需要调大 |
| 4.7 | 半开连接：一端已断但另一端不知 | ⚠️ 依赖 keepalive | yamux keepalive 最终会检测到 |
| 4.8 | Edge 所在网络 NAT 超时切换 IP → TCP 连接断开 | ⚠️ 被动重连 | Edge 5s 重连循环 |
| 4.9 | Bridge 公网 IP 变化 → 所有 Edge/Client 隧道断开 | ⚠️ 被动重连 | 需要域名 + DNS 更新 |

## 5. 并发与竞态

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 5.1 | 多个 SOCKS5 请求并发 → 共用同一 yamux session 并行开 stream | ✅ 已实现 | yamux 原生支持 |
| 5.2 | 首次请求触发 ensureTunnel 时多个请求同时到达 → 重复建连 | ✅ 已实现 | mutex + 二次检查 |
| 5.3 | ensureTunnel 锁外做 HTTP 调用 → 其他请求阻塞 | ✅ 已实现 | 锁外做 I/O |
| 5.4 | heartbeat 与 ensureTunnel 并发访问 tunnelState | ⚠️ 潜在问题 | heartbeat 直接访问 `s.tunnelState` 而非 `getTunnelState()` |
| 5.5 | Edge 断线重连 → Bridge 收到新 session → 旧 session 上的活跃 stream 被关闭 | ⚠️ 活跃请求中断 | `old.Close()` 会断开正在转发的流 |
| 5.6 | 两个 Client 同时请求同一个 idle Edge → 都尝试绑定 | ❌ 未处理 | 无数据库级别的乐观锁 |
| 5.7 | binding_cleanup 与 heartbeat 并发修改 node status | ⚠️ 潜在冲突 | 依赖数据库事务 |
| 5.8 | Client 心跳 edge_id="" 与实际已建连的 tunnel 状态不一致 | ⚠️ 窗口期 | 首次心跳在 tunnel 建立前发出 |

## 6. 资源耗尽

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 6.1 | yamux stream 数量爆炸（大量并发请求） | ⚠️ 无显式限制 | yamux 默认无 stream 上限（Client 侧） |
| 6.2 | Edge 连接数达到 max_connections → 新请求被静默丢弃 | ⚠️ 部分 | `allowConn` 返回 false 但未回复错误 |
| 6.3 | Edge 内存超过 max_memory_mb（Android） | ❌ 未实现 | 配置存在但未读取/使用 |
| 6.4 | Bridge 大量死 edgeSession 占用内存 | ❌ 未处理 | 无 GC 机制清理断开的 session |
| 6.5 | Client 端文件描述符耗尽（大量 SOCKS5 连接） | ⚠️ OS 限制 | 无应用层限流 |
| 6.6 | 慢速 Client 导致 Bridge/Edge 缓冲区积压（backpressure） | ✅ 已实现 | yamux + TCP 自带流控 |
| 6.7 | 目标服务器响应极慢 → stream 长期占用 | ⚠️ 无超时 | Edge 无请求级别超时 |
| 6.8 | 大量短连接导致 TIME_WAIT 堆积（Edge 侧） | ⚠️ OS 参数 | 需调优 sysctl |
| 6.9 | PostgreSQL 连接池耗尽（高并发 API 请求） | ⚠️ GORM 默认 | 未显式设置池大小 |

## 7. 认证与权限

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 7.1 | Client token 无效 → auth 失败 → userID=0 → 能转发但不计费 | ⚠️ 不安全 | auth 失败应阻止建隧道 |
| 7.2 | Edge token 无效 → 注册失败 → 但仍可建隧道到 Bridge | ❌ 未处理 | Bridge 隧道无认证 |
| 7.3 | 伪造 EDGE/CLIENT 首流注册 → 恶意节点接入 | ❌ 未处理 | 隧道层无 token 验证 |
| 7.4 | 重放 CONNECT 请求中的 user_id → 冒充他人计费 | ❌ 未处理 | Bridge 不验证 user_id 合法性 |
| 7.5 | Client auth 返回的 user_id 与 CONNECT 携带的不一致 | ❌ 未处理 | Bridge 信任 Client 自报的 user_id |
| 7.6 | Edge ID 冲突 → 两个 Edge 用相同 ID 注册 | ⚠️ 后者替换前者 | `old.Close()` 踢掉旧 Edge |

## 8. 计费系统全场景

> **计费链路**：Bridge countConn 统计字节 → stream 关闭 → reportTraffic POST → apiHub 写 traffic_stats → 冻结余额(FrozenBalance) → 10分钟定时结算(settle) → 扣 user.balance → 写 fund_flows 流水
>
> **计量点**：Bridge 的 `countConn` 包装 client yamux stream，统计双向裸数据字节（不含 CONNECT 协议头）
>
> **两条计费路径**：有 MQ → 异步冻结（message_tracking 幂等）；无 MQ → 同步冻结（无幂等保护）

### 8A. 数据采集层（Bridge countConn）

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 8A.1 | 正常请求 → stream 关闭后上报 bytes_in/bytes_out | ✅ 已实现 | done channel 同步后上报 |
| 8A.2 | 请求中途断开 → 只统计到断开为止的字节 | ✅ 已实现 | io.Copy 返回即停止计数 |
| 8A.3 | 统计范围：只计裸数据，不含 CONNECT 协议头和 OK 响应 | ✅ 正确 | 协议头由 bufio.Reader 消费后直接转发 |
| 8A.4 | bytes_in/bytes_out 方向语义：in=用户下载(edge→client)，out=用户上传(client→edge) | ✅ 已实现 | 代码注释已标明 |
| 8A.5 | 极大文件传输（> 1GB）→ int64 计数是否溢出 | ✅ 不会 | int64 最大 8EB |
| 8A.6 | 零字节传输（连接后立即关闭） → bytes_in=0 bytes_out=0 | ✅ 会上报 | 写入 traffic_stats 记录 |
| 8A.7 | countConn 的 mutex 是否成为性能瓶颈 | ⚠️ 可优化 | 每次 Read/Write 都加锁；可改用 atomic |
| 8A.8 | yamux 帧开销是否被计入流量 | ✅ 未计入 | countConn 包装的是 yamux stream 层面，yamux 帧头在底层处理 |

### 8B. 流量上报层（Bridge → apiHub）

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 8B.1 | userID=0（auth 失败或跳过）→ 不上报 → 免费白嫖 | ❌ 安全漏洞 | auth 失败应阻止建隧道 |
| 8B.2 | userID 由 Client 自行携带 → Bridge 信任不验证 | ❌ 安全漏洞 | Client 可伪造 userID 嫁祸他人 |
| 8B.3 | apiHub 不可达 → http.Post 失败 → 数据丢失无重试 | ❌ 未处理 | 需本地队列 + 重试 |
| 8B.4 | apiHub 返回 500 → reportTraffic 只 Close body → 数据丢失 | ❌ 未处理 | 未检查 resp.StatusCode |
| 8B.5 | 上报延迟：只在 stream 完全关闭后才上报 → 长连接(WebSocket)期间无中间上报 | ❌ 未处理 | 长连接可能持续数小时，崩溃则丢失全部数据 |
| 8B.6 | Bridge 崩溃 → 内存中的 upBytes/downBytes 全部丢失 | ❌ 未处理 | 无持久化中间状态 |
| 8B.7 | 网络超时 → http.Post 默认无超时 → 可能阻塞很久 | ❌ 未处理 | 无 http.Client 超时设置 |
| 8B.8 | 高并发上报 → apiHub 串行写 DB → 可能成为瓶颈 | ⚠️ 潜在 | 无批量上报机制 |
| 8B.9 | 同一 stream 被重复上报（理论上不会，但无幂等保护） | ⚠️ 无幂等 | traffic_stats 无 unique 约束 |

### 8C. 流量记录层（apiHub traffic_stats）

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 8C.1 | 每条 stream 对应一条 traffic_stats 记录 | ✅ 已实现 | |
| 8C.2 | traffic_stats 无 unique 约束 → 重复写入不报错 | ⚠️ 需改进 | 应加唯一标识（如 stream_id） |
| 8C.3 | traffic_stats 只记录 user_id + edge_id → 不知道具体访问了什么目标 | ⚠️ 设计选择 | 隐私考虑可能不记录目标 |
| 8C.4 | traffic_stats 表无分区/归档 → 数据量增长后查询变慢 | ❌ 未处理 | 需按时间分区或定期归档 |
| 8C.5 | reported_at 为 Bridge 发起上报的时间，非实际使用时间 | ⚠️ 轻微偏差 | 长连接偏差可能很大 |

### 8D. 冻结余额层（FrozenBalance）

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 8D.1 | **MQ 路径**：每条消息生成一条 FrozenBalance + 更新 user.frozen_balance | ✅ 已实现 | MQ consumer |
| 8D.2 | **同步路径（无 MQ）**：handleTrafficReport 直接冻结 | ✅ 已实现 | else 分支 |
| 8D.3 | 计费精度：`amount = totalBytes / (1024*1024)` 整数除法 → 不足 1MB 的部分被丢弃 | ⚠️ 精度丢失 | 每个请求独立截断，大量小请求永远不计费 |
| 8D.4 | MQ 路径有 1MB 阈值过滤（consumeTraffic）→ 小请求不冻结 | ⚠️ 与同步路径一致 | 但 traffic_stats 已记录，仅冻结跳过 |
| 8D.5 | 同步路径的 refID=时间戳 → 同一秒内多次上报 refID 相同 → 无法区分 | ❌ 有问题 | refID 不唯一 |
| 8D.6 | MQ 路径的 message_tracking 有 uniqueIndex → 可防重复消费 | ✅ 已实现 | |
| 8D.7 | 但 MQ 路径中 Publish 失败 → message_tracking 为 pending → 永不消费 → 永不冻结 | ❌ 未处理 | 无重试/补偿机制 |
| 8D.8 | frozen_balance 累加用 `gorm.Expr("frozen_balance + ?")` → DB 原子操作 | ✅ 已实现 | 并发安全 |
| 8D.9 | 冻结但最终不结算（如用户退款/充值后余额足够抵消）→ 冻结金额一直挂着 | ⚠️ 设计如此 | settle 最终会结算 |

### 8E. 定时结算层（Settle Loop，每 10 分钟）

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 8E.1 | 收集所有 settled_at IS NULL 的 FrozenBalance → 按 user 汇总扣款 | ✅ 已实现 | DB 事务 |
| 8E.2 | 余额不足时：`newBalance = u.Balance - total; if < 0 → 0` | ⚠️ 不合理 | 余额被截为 0，差额损失（运营商亏损） |
| 8E.3 | 冻结余额不足时：`newFrozen = u.FrozenBalance - total; if < 0 → 0` | ⚠️ 可能不一致 | 实际冻结金额可能与 sum(FrozenBalance) 不匹配 |
| 8E.4 | FundFlow 记录 amount=-total, balance=newBalance → 但 newBalance 被截为 0 → 审计不准确 | ❌ 有问题 | 实际扣款额 ≠ amount，FundFlow 不反映真实扣减 |
| 8E.5 | settle 事务失败 → 全部回滚 → 下次重试 | ✅ 已实现 | 事务原子性 |
| 8E.6 | settle 运行期间新的 FrozenBalance 写入 → 本轮不处理 | ✅ 正确 | WHERE settled_at IS NULL 在查询时快照 |
| 8E.7 | 多个 apiHub 实例同时 settle → 重复扣款 | ❌ 未处理 | 无分布式锁 |
| 8E.8 | settle 间隔 10 分钟 → 用户看到的余额变化有延迟 | ⚠️ 设计如此 | 实时性不高 |
| 8E.9 | 大量用户同时 settle → 单个大事务 → 可能锁表 | ⚠️ 潜在 | 应分批 settle |
| 8E.10 | FundFlow.RefID 固定为 "settle" → 无法追溯对应哪些 FrozenBalance | ⚠️ 需改进 | 应关联 settle 批次 ID |

### 8F. 用户余额与使用控制

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 8F.1 | 新用户 balance=0 → 无充值即可使用 → 无限免费 | ❌ 未处理 | 无余额预检查 |
| 8F.2 | 用户余额查询（/api/user/balance）→ 返回 balance 不含 frozen_balance | ⚠️ 可能误导 | 用户看到的余额未扣除冻结部分 |
| 8F.3 | 充值流程 → 无充值 API | ❌ 未实现 | 需 POST /api/user/recharge |
| 8F.4 | 退款流程 → 无退款 API | ❌ 未实现 | |
| 8F.5 | 余额预扣 → 请求前检查 balance - frozen_balance ≥ 0 | ❌ 未实现 | 防止欠费使用 |
| 8F.6 | 余额耗尽后断开连接 → 目前无任何限制 | ❌ 未实现 | 用户可无限使用 |
| 8F.7 | 账户冻结/禁用 → 目前无此机制 | ❌ 未实现 | users 表无 disabled 字段 |

### 8G. 计费数据一致性与对账

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 8G.1 | traffic_stats 总量 vs sum(FrozenBalance.amount) → 可能不一致 | ❌ 有差异 | 每条记录独立截断 MB |
| 8G.2 | sum(FrozenBalance.amount) vs user.frozen_balance → 可能不一致 | ❌ 有差异 | settle 后 frozen_balance 可能被截为 0 |
| 8G.3 | FundFlow 流水 vs 实际 balance 变动 → 余额截为 0 时不一致 | ❌ 有差异 | 见 8E.4 |
| 8G.4 | 对账脚本/API → 不存在 | ❌ 未实现 | 需定期校验数据一致性 |
| 8G.5 | 异常数据修正 → 无管理后台/API | ❌ 未实现 | 运营无法手动修正 |

### 8H. 计费安全

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 8H.1 | Client 伪造 user_id=0 → 免计费使用 | ❌ 安全漏洞 | Bridge 应从 session 关联 user_id，而非信任 Client |
| 8H.2 | Client 伪造 user_id=其他用户 → 消耗他人余额 | ❌ 安全漏洞 | 同上 |
| 8H.3 | 恶意 Client 故意不关闭 stream → 延迟计费 → 长期免费使用 | ⚠️ 无限制 | 长连接无中间上报 |
| 8H.4 | 伪造 reportTraffic API 请求 → apiHub 无鉴权 → 可注入虚假计费数据 | ❌ 安全漏洞 | /api/traffic/report 无认证 |
| 8H.5 | 伪造 reportTraffic 写入巨额 bytes → 恶意消耗他人余额 | ❌ 安全漏洞 | 同上 |
| 8H.6 | Bridge 被入侵 → 可操控所有计费数据 | ⚠️ 信任模型 | Bridge 是可信组件 |

### 8I. 长连接与特殊流量模式

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 8I.1 | WebSocket 长连接 → 数小时后才上报流量 → 崩溃则全部丢失 | ❌ 未处理 | 需定时中间上报 |
| 8I.2 | 视频流/直播 → 持续大流量 → 单次上报金额巨大 → 冻结余额突增 | ⚠️ 无限制 | 应有单次上报金额上限 |
| 8I.3 | 大量并发小请求（爬虫场景）→ 每个 < 1MB → 永不计费 | ❌ 计费漏洞 | 应累积到阈值再计费 |
| 8I.4 | 下载超大文件（> 10GB）→ 全部在一条 stream → 单次上报 | ⚠️ 延迟计费 | 崩溃风险大 |
| 8I.5 | TCP keep-alive 空闲连接 → 占 stream 但无流量 → 不产生计费 | ✅ 正确 | 零字节不收费 |

## 9. 状态管理（Node Status: offline/idle/busy）

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 9.1 | Edge 注册 → idle | ✅ 已实现 | |
| 9.2 | Client 绑定 Edge → busy | ✅ 已实现 | heartbeat 中设置 |
| 9.3 | Client 解绑 → idle | ✅ 已实现 | heartbeat edge_id="" |
| 9.4 | Client 换绑 → 旧 edge idle，新 edge busy | ✅ 已实现 | |
| 9.5 | Client 崩溃 → binding_cleanup 清理 → edge 回 idle | ✅ 已实现 | 2 分钟超时 |
| 9.6 | Edge 心跳将 offline 恢复为 idle，但不覆盖 busy | ✅ 已实现 | CASE WHEN |
| 9.7 | Client 重启后 Edge 仍为 busy → Client 能看到已绑定的 Edge | ✅ 已实现 | LEFT JOIN binding 查询 |
| 9.8 | Edge 下线 → binding_cleanup 清绑定 → 但 Client 不知道 Edge 已断 | ⚠️ 延迟感知 | Client 下次请求才发现 |
| 9.9 | 多个 Client 尝试绑定同一 Edge（并发竞态） | ❌ 未处理 | 无乐观锁/CAS |
| 9.10 | Edge busy 但实际无流量（Client 已空闲很久） | ⚠️ 设计如此 | 解绑需 Client 主动或超时 |

## 10. 多路复用场景（yamux / QUIC）

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 10.1 | 单 session 上百个并发 stream → 性能 | ✅ yamux/QUIC 设计 | 两者均可处理大量 stream |
| 10.2 | session 心跳超时 → session 关闭 → 所有 stream 中断 | ⚠️ 预期行为 | Client/Edge 需重连 |
| 10.3 | 单条 stream 阻塞不影响其他 stream（yamux: 应用层流控；QUIC: 协议层流控） | ✅ 已支持 | QUIC 更彻底（无 TCP 队头阻塞） |
| 10.4 | **TCP+yamux 队头阻塞**：底层 TCP 丢包 → 所有 stream 等重传 → QUIC 无此问题 | ⚠️ TCP 固有 | QUIC 每个 stream 独立恢复 |
| 10.5 | 底层连接 reset → 整个 session 挂掉（TCP: RST；QUIC: CONNECTION_CLOSE） | ⚠️ 预期行为 | 需重连 |
| 10.6 | stream 未关闭导致资源泄漏 | ⚠️ 需检查 | defer Close 已覆盖 |
| 10.7 | yamux window size 默认 256KB → 单流大文件下载可能受限 | ⚠️ 可能瓶颈 | 可调 yamux.Config；QUIC 窗口自适应 |
| 10.8 | **QUIC 模式下无需 yamux** → 直接使用 QUIC 原生 stream | ❌ 未实现 | 见第 15 节 |

## 11. Edge Android 特有场景

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 11.1 | Android Doze 模式 → 网络受限 → 心跳/隧道中断 | ❌ 未处理 | 需 WakeLock 或 Foreground Service |
| 11.2 | 移动网络切换（WiFi↔4G）→ IP 变化 → TCP 断开 | ⚠️ 被动重连 | 5s 重连循环 |
| 11.3 | OOM Killer 杀死 Edge 进程 | ❌ 未处理 | 需 Foreground Service 提高优先级 |
| 11.4 | 32 位 ARMv7 内存限制 → max_memory_mb 配置未实际使用 | ❌ 未实现 | 配置存在但无运行时检查 |
| 11.5 | 低电量模式 → CPU 降频 → 转发性能下降 | ⚠️ 无法控制 | low_power_mode 配置未使用 |
| 11.6 | Android DNS 解析限制 → 部分域名无法解析 | ⚠️ 系统行为 | 取决于 Android 系统 |
| 11.7 | Android 后台执行限制 → 进程可能被系统暂停 | ❌ 未处理 | 需 Service 机制 |
| 11.8 | 存储空间不足 → 日志写入失败 | ⚠️ 无影响 | 日志写 stdout |

## 12. 多 Bridge 集群场景

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 12.1 | Client 和 Edge 连到不同 Bridge → 无法转发 | ❌ 未处理 | 需 Bridge 间通信或统一调度 |
| 12.2 | 同一 Edge 连到多个 Bridge → Bridge A 不知道 Bridge B 的 Edge | ❌ 未处理 | Edge 设计为单 Bridge 连接 |
| 12.3 | Bridge 负载均衡 → Client/Edge 如何选择 Bridge | ❌ 未实现 | 配置文件写死 Bridge 地址 |
| 12.4 | Bridge 间心跳同步 Edge 列表 | ❌ 未实现 | 当前只有单 Bridge |

## 13. 部署与运维

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 13.1 | Bridge 滚动升级 → Edge/Client 断连后自动重连 | ✅ 已实现 | |
| 13.2 | apiHub 滚动升级 → 短暂 API 不可用 → 不影响已建立隧道 | ✅ 已实现 | |
| 13.3 | 数据库迁移 → 需停机或在线 DDL | ⚠️ 手动 | AutoMigrate 在启动时 |
| 13.4 | 日志轮转 → 所有组件写 stdout/stderr | ✅ 已实现 | 由外部管理（systemd/docker） |
| 13.5 | 监控 → 无 Prometheus metrics | ❌ 未实现 | 无可观测性 |
| 13.6 | 配置热更新 → 不支持 | ❌ 未实现 | 需重启 |

## 14. 协议边界与异常输入

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 14.1 | 恶意 SOCKS5 客户端发送超长域名 | ⚠️ 受 buf 大小限制 | buf 256 字节 |
| 14.2 | CONNECT 请求中 edge_id 不存在 → stream 静默关闭 | ⚠️ 无错误响应 | Client 超时 |
| 14.3 | CONNECT 请求格式错误（缺少 edge_id 或 host:port） | ✅ 已实现 | 解析失败直接关闭 |
| 14.4 | 极大 HTTP 响应（> 1GB）通过隧道 | ✅ 已实现 | io.Copy 流式转发 |
| 14.5 | WebSocket 长连接通过 SOCKS5 | ✅ 已实现 | 双向转发直到一端关闭 |
| 14.6 | HTTPS CONNECT（TLS 透传） | ✅ 已实现 | SOCKS5 CONNECT 建立隧道后 TLS 由端到端处理 |
| 14.7 | 目标端口被防火墙封锁 → Edge net.Dial 超时 | ⚠️ 系统默认 | 无自定义 Dial 超时 |
| 14.8 | 首流非 EDGE/CLIENT → session 被关闭 | ✅ 已实现 | handleSession 兜底 |

## 15. 传输层：QUIC 优先 + TCP 降级

> **目标架构**：Client/Edge 连接 Bridge 时先尝试 QUIC（UDP），失败则降级到 TCP+yamux。
>
> **QUIC vs TCP+yamux 关键差异**：
> - QUIC 基于 UDP，内建 TLS 1.3 加密 + 原生多路复用 + 连接迁移
> - TCP+yamux 基于 TCP，当前无加密，yamux 提供多路复用
> - QUIC 无队头阻塞（单流丢包不影响其他流），TCP 有
> - QUIC 支持 0-RTT 快速重连，TCP 需三次握手

### 15A. 连接建立与降级策略

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 15A.1 | QUIC 拨号成功 → 使用 QUIC 原生 stream（无需 yamux） | ❌ 未实现 | 需 quic-go 集成 |
| 15A.2 | QUIC 拨号失败（UDP 被防火墙封锁）→ 立即降级到 TCP+yamux | ❌ 未实现 | 需降级逻辑 |
| 15A.3 | QUIC 拨号超时（UDP 被限速/丢弃）→ 等多久才降级？ | ❌ 未实现 | 建议 3-5 秒超时后降级 |
| 15A.4 | Happy Eyeballs 并行尝试：QUIC 和 TCP 同时拨号，谁先成功用谁 | ❌ 未实现 | 减少降级延迟（推荐方案） |
| 15A.5 | QUIC 握手成功但后续传输不稳定（频繁超时/重传）→ 是否运行中降级 | ❌ 未实现 | 检测质量劣化后主动切换 |
| 15A.6 | 降级到 TCP 后，是否定期重试 QUIC → 重试间隔策略 | ❌ 未实现 | 指数退避重试（如 1min → 5min → 30min） |
| 15A.7 | 降级结果缓存：同一 Bridge 地址降级后缓存多久 | ❌ 未实现 | 避免每次连接都重试失败的 QUIC |
| 15A.8 | 配置强制协议：`transport: "auto" / "quic" / "tcp"` | ❌ 未实现 | auto 为默认 |
| 15A.9 | QUIC 降级到 TCP 时正在进行的请求如何处理 | ❌ 需设计 | 降级发生在连接层，活跃请求随旧连接中断后重试 |

### 15B. Bridge 双协议监听

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 15B.1 | Bridge 同时监听 UDP（QUIC）+ TCP（yamux）→ 两个 Listener | ❌ 未实现 | 需双 goroutine |
| 15B.2 | 同端口 vs 不同端口：QUIC(UDP:8081) + TCP(TCP:8081) 可共用端口号 | ❌ 需设计 | UDP 和 TCP 是不同协议，端口号可复用 |
| 15B.3 | Bridge 统一 session 管理：不区分 QUIC session 和 yamux session | ❌ 未实现 | 需抽象 `MuxSession` 接口 |
| 15B.4 | QUIC Listener 崩溃但 TCP Listener 正常 → 仅 QUIC 客户端降级 | ❌ 未实现 | 两个 Listener 独立 |
| 15B.5 | Bridge 如何告知 Client/Edge 支持哪些协议 → 协议发现 | ❌ 未实现 | 可在 /api/edges 返回 `protocols: ["quic","tcp"]` |
| 15B.6 | 混合模式：Client 用 QUIC → Bridge → Edge 用 TCP（或反之） | ❌ 需设计 | Bridge 内部转发不感知底层协议 |

### 15C. QUIC 特有优势场景

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 15C.1 | **连接迁移**：Edge 手机 WiFi↔4G 切换 → QUIC 连接不断 → 无需重连 | ❌ 未实现 | TCP 下必须重连，QUIC 原生支持 |
| 15C.2 | **0-RTT 快速重连**：Bridge 重启后 Edge/Client 极速恢复 | ❌ 未实现 | QUIC 支持 0-RTT 握手 |
| 15C.3 | **无队头阻塞**：单 stream 丢包不影响其他 stream | ❌ 未实现 | TCP 下丢一个包所有 stream 等待重传 |
| 15C.4 | **内建加密**：QUIC = TLS 1.3 → 不需要额外 TLS 层 | ❌ 未实现 | 当前 TCP 无加密 |
| 15C.5 | **拥塞控制**：QUIC 可选用 BBR → 高丢包网络表现更好 | ❌ 未实现 | 取决于 quic-go 配置 |
| 15C.6 | 高丢包移动网络 → QUIC 比 TCP 更稳定 | ❌ 未实现 | QUIC 的核心价值之一 |

### 15D. QUIC 特有风险场景

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 15D.1 | **UDP 被防火墙封锁**（企业网络、学校、部分国家）→ 必须降级 | ❌ 需降级 | 降级策略是必需的 |
| 15D.2 | **UDP 被 ISP 限速**（某些运营商限制 UDP 带宽）→ QUIC 比 TCP 慢 | ❌ 需检测 | 需质量探测后决定是否降级 |
| 15D.3 | **NAT 对 UDP 超时更短**（通常 30s vs TCP 的 120s+）→ 空闲连接易断 | ❌ 需处理 | QUIC keepalive 间隔需 < NAT 超时 |
| 15D.4 | **QUIC TLS 证书管理**：自签证书 vs Let's Encrypt vs 内嵌证书 | ❌ 需设计 | 生产环境需要正式证书链 |
| 15D.5 | **TLS 加密 CPU 开销**：Android 低端设备（ARMv7）可能成为瓶颈 | ❌ 需评估 | 可能需要根据设备能力选择协议 |
| 15D.6 | **quic-go 库成熟度和兼容性**：Go 生态主要依赖 quic-go | ⚠️ 需评估 | 确认与 Go 1.24 兼容性 |
| 15D.7 | **QUIC 版本协商**：Client/Bridge QUIC 版本不一致 | ❌ 需处理 | QUIC 有版本协商机制 |
| 15D.8 | **MTU 探测问题**：某些网络 Path MTU Discovery 受阻 → 大包被丢 | ⚠️ QUIC 处理 | quic-go 内建 PMTUD |
| 15D.9 | **QUIC 放大攻击**：Bridge 的 UDP 端口可被用于反射攻击 | ⚠️ 需防护 | QUIC 有 Initial 包大小限制，但仍需注意 |

### 15E. 降级策略详细设计

| # | 场景 | 建议方案 |
|---|------|---------|
| 15E.1 | **首次连接**：QUIC 3s 超时 → 降级 TCP；或 Happy Eyeballs 并行（推荐） | QUIC 先发，TCP 延迟 300ms 后发，谁先成功用谁 |
| 15E.2 | **降级缓存**：记住 `(bridge_addr, protocol)` → 下次直接用已知可行协议 | TTL 5 分钟（短时间缓存，允许网络恢复后回 QUIC） |
| 15E.3 | **运行中切换**：QUIC 连接质量劣化 → 新建 TCP 连接 → 迁移 session | 旧 QUIC session drain（等活跃 stream 结束）→ 新 stream 走 TCP |
| 15E.4 | **Edge 重连循环**：`runTunnel` 内应支持 QUIC→TCP 降级 | `for { tryQUIC(); if fail { tryTCP() }; sleep 5s }` |
| 15E.5 | **Client ensureTunnel**：同理支持协议选择 | 与 Edge 相同的降级逻辑 |
| 15E.6 | **协议探测日志**：记录使用了哪种协议，方便排障 | `log.Printf("tunnel: connected via QUIC/TCP")` |

### 15F. 抽象层设计

| # | 场景 | 需要处理 |
|---|------|---------|
| 15F.1 | 统一 Session 接口：`type MuxSession interface { Open() (stream, error); Accept() (stream, error); Close() error }` | yamux.Session 和 quic.Connection 都实现此接口 |
| 15F.2 | 统一 Stream 接口：`net.Conn` 已满足（QUIC stream 和 yamux stream 都实现） | 无需额外抽象 |
| 15F.3 | 统一 Dialer：`func Dial(addr string, transport string) (MuxSession, error)` | 封装 QUIC/TCP 拨号逻辑 |
| 15F.4 | 统一 Listener：`func Listen(addr string) (accepts both QUIC and TCP)` | Bridge 端双协议监听 |
| 15F.5 | 现有 `pkg/tunnel` 包需要重构为传输层抽象 | 当前 `tunnel.Dial/Listen` 仅支持 TCP+yamux |

### 15G. 对计费系统的影响

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 15G.1 | QUIC stream 和 yamux stream 的 countConn 逻辑相同 → 计费不受协议影响 | ✅ 无影响 | countConn 包装 net.Conn 接口 |
| 15G.2 | QUIC 重传开销不应计入用户流量 → countConn 在 stream 层，不含重传 | ✅ 正确 | 与 TCP 重传同理 |
| 15G.3 | QUIC TLS 加密开销不被计入 → countConn 在解密后的 stream 层 | ✅ 正确 | |
| 15G.4 | 协议切换时流量连续性 → 旧连接上报 + 新连接重新开始 | ✅ 自然处理 | 每条 stream 独立上报 |
| 15G.5 | QUIC vs TCP 传输效率差异 → 同样数据量，网络层实际消耗不同 | ⚠️ 设计选择 | 是否按用户数据量计费（推荐）还是按网络消耗 |

### 15H. Android Edge 特有

| # | 场景 | 当前状态 | 需要处理 |
|---|------|---------|---------|
| 15H.1 | QUIC 连接迁移 → WiFi↔4G 无缝切换 → 不中断转发 | ❌ 未实现 | QUIC 最大优势之一 |
| 15H.2 | 移动运营商限制 UDP → 部分 4G 网络 QUIC 不可用 | ❌ 需降级 | 自动降级到 TCP |
| 15H.3 | QUIC TLS 握手的 CPU/电量消耗 → 低端设备可能明显 | ❌ 需评估 | 必要时强制 TCP 模式 |
| 15H.4 | Android VPN Service 拦截 UDP → QUIC 可能被干扰 | ⚠️ 环境依赖 | 需测试 |
| 15H.5 | QUIC keepalive 可维持 NAT 映射 → 比 TCP 更适合移动网络 | ❌ 未实现 | 前提是 keepalive 间隔正确 |

---

## 优先修复清单

### P0 — 影响核心功能 / 计费安全

| # | 问题 | 来源 |
|---|------|------|
| P0-1 | Bridge 无法清理死亡的 edgeSession → Open 一直失败 → 所有到该 Edge 的请求失败 | 3.2.2, 6.4 |
| P0-2 | Edge 连接数满时 `allowConn` 返回 false 但未回复错误 → Client 超时 | 6.2 |
| P0-3 | Client auth 失败后仍能建隧道转发 → 白嫖流量 | 7.1, 8B.1 |
| P0-4 | heartbeat 直接访问 `s.tunnelState` 而非 `getTunnelState()` → 潜在 nil panic | 5.4 |
| P0-5 | **user_id 由 Client 自报 → 可伪造为 0（免费）或他人 ID（嫁祸）** | 8H.1, 8H.2, 8B.2 |
| P0-6 | **/api/traffic/report 无鉴权 → 可伪造计费数据消耗他人余额** | 8H.4, 8H.5 |

### P1 — 影响计费可靠性

| # | 问题 | 来源 |
|---|------|------|
| P1-1 | Edge 崩溃后 Client 不会自动换绑新 Edge → 持续失败直到手动重启 | 3.2.3 |
| P1-2 | **流量上报失败无重试 → 计费数据永久丢失** | 3.1.4, 3.4.4, 8B.3 |
| P1-3 | 两个 Client 同时绑定同一 Edge → 状态不一致 | 5.6, 9.9 |
| P1-4 | Edge→Target 无 Dial 超时 → 慢目标阻塞 stream | 6.7, 14.7 |
| P1-5 | 隧道层无认证 → 任何人可连 Bridge 伪装 Edge/Client | 7.2, 7.3 |
| P1-6 | **长连接(WebSocket)只在 stream 关闭后才上报 → 崩溃丢失全部数据** | 8B.5, 8I.1, 8I.4 |
| P1-7 | **大量小请求(< 1MB)永不计费 → 爬虫场景完全免费** | 8D.3, 8I.3 |
| P1-8 | **reportTraffic 未检查 resp.StatusCode → apiHub 报错也当成功** | 8B.4 |
| P1-9 | **reportTraffic 的 http.Post 无超时 → 可能阻塞** | 8B.7 |

### P2 — 影响计费准确性

| # | 问题 | 来源 |
|---|------|------|
| P2-1 | yamux window size 默认 256KB → 大文件传输吞吐受限 | 10.7 |
| P2-2 | max_memory_mb / low_power_mode 配置未实现 | 11.4, 11.5 |
| P2-3 | Client 无 SOCKS5 连接数限制 → 可能耗尽 fd | 6.5 |
| P2-4 | 无 Prometheus metrics → 无法监控 | 13.5 |
| P2-5 | CONNECT edge 不存在时无错误响应 → Client 等到超时 | 14.2 |
| P2-6 | **settle 余额不足截为 0 → FundFlow 记录的 amount 与实际扣减不一致 → 审计不准** | 8E.2, 8E.4, 8G.3 |
| P2-7 | **同步路径 FrozenBalance.RefID=时间戳 → 同秒多条不可区分** | 8D.5 |
| P2-8 | **settle FundFlow.RefID="settle" → 无法追溯对应哪批 FrozenBalance** | 8E.10 |
| P2-9 | **countConn 每次 Read/Write 加 mutex → 高并发性能开销** | 8A.7 |
| P2-10 | **用户余额查询不含冻结部分 → 用户看到的"可用余额"不准确** | 8F.2 |

### P3 — 传输层升级（QUIC）

| # | 问题 | 来源 |
|---|------|------|
| P3-1 | **`pkg/tunnel` 抽象为 MuxSession 接口 → 同时支持 QUIC 和 TCP+yamux** | 15F.1, 15F.5 |
| P3-2 | **Bridge 双协议监听（UDP+TCP 同端口号）** | 15B.1, 15B.2 |
| P3-3 | **Client/Edge QUIC 优先 + TCP 降级拨号逻辑** | 15A.1, 15A.2, 15E.4, 15E.5 |
| P3-4 | **Happy Eyeballs 并行拨号（QUIC 先发，TCP 延迟 300ms）** | 15A.4, 15E.1 |
| P3-5 | **降级结果缓存（避免反复尝试失败的 QUIC）** | 15A.7, 15E.2 |
| P3-6 | **QUIC TLS 证书方案（自签 / ACME / 内嵌）** | 15D.4 |
| P3-7 | **协议探测日志 + 可观测性** | 15E.6 |

### P4 — 增强与优化

| # | 问题 | 来源 |
|---|------|------|
| P4-1 | **用户余额预检查 → 余额不足时拒绝新请求** | 8F.1, 8F.5, 8F.6 |
| P4-2 | Client/Edge graceful shutdown（信号处理） | 2.3, 2.4 |
| P4-3 | 多 Bridge 集群支持 | 12.x |
| P4-4 | 配置热更新 | 13.6 |
| P4-5 | Android Doze / Foreground Service 支持 | 11.1, 11.3, 11.7 |
| P4-6 | **流量上报幂等（traffic_stats 加 stream_id 唯一约束）** | 8B.9, 8C.2 |
| P4-7 | **traffic_stats 表分区/归档策略** | 8C.4 |
| P4-8 | **充值/退款 API** | 8F.3, 8F.4 |
| P4-9 | **对账脚本/管理后台** | 8G.4, 8G.5 |
| P4-10 | **账户冻结/禁用机制** | 8F.7 |
| P4-11 | **MQ message_tracking 补偿：pending 消息重投** | 8D.7 |
| P4-12 | **settle 分批处理避免大事务锁表** | 8E.9 |

---

*Last updated: 2026-03-13 (v3: 新增 QUIC+TCP 降级场景)*
