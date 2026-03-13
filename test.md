# proxy-bridge 测试场景

> 聚焦两大核心功能：**转发** 和 **计费**
>
> **标记说明**：`[E]` 端到端测试 · `[I]` 集成测试 · `[S]` 压力测试

---

## 数据库 Schema 速查

| 表 | 关键字段 | 说明 |
|----|----------|------|
| `users` | `id`, `username`, `token`, `balance` | 用户账户，balance 单位 KB |
| `nodes` | `id`, `node_type`, `node_id`, `token`, `user_id`, `country`, `status`, `last_seen` | 统一节点表 |
| `client_edge_bindings` | `id`, `client_id`, `edge_id` | Client-Edge 绑定 |
| `edge_registrations` | `id`, `edge_id`, `node_id`, `addr`, `country`, `last_seen` | Edge 注册 |
| `bridge_registrations` | `id`, `bridge_id`, `addr`, `last_seen` | Bridge 注册 |
| `traffic_stats` | `id`, `user_id`, `edge_id`, `bytes_in`, `bytes_out`, `reported_at` | 流量统计 |
| `fund_flows` | `id`, `user_id`, `amount`, `balance`, `type` | 资金流水 |

## 计费规则

- **单位**：1 KB = 1 单位余额
- **计算**：`cost = (bytes_in + bytes_out) / 1024`（整除，不足 1KB 不收费）
- **扣款时机**：每次连接关闭后 Bridge 上报流量，apiHub 立即从 `users.balance` 扣除
- **余额检查**：Bridge 每次转发前查询余额，`balance <= 0` 时拒绝转发
- **流水记录**：每次扣款写入 `fund_flows`（amount 为负数，balance 为扣后快照）

## 初始状态（seed 后）

```sql
-- users: balance=10000（初始 10000 KB 额度）
-- nodes: client(online), edge(idle), user_id 已关联
-- 其他表均为空
```

---

## T1. 转发 — 正常流程

### T1.1 全链路 SOCKS5 转发 `[E]`
- **步骤**：`curl -x socks5h://127.0.0.1:1080 http://httpbin.org/ip`
- **断言（网络）**：HTTP 200，body 含 `origin`
- **断言（DB）**：
  - `traffic_stats`: 新增 1 条，`user_id>0`, `edge_id='edge-1'`, `bytes_in>0`, `bytes_out>0`
  - `nodes(edge-1)`: `status='busy'`, `country='cn'`, `last_seen` 近 2 分钟
  - `nodes(client-e2e)`: `status='online'`, `last_seen` 近 2 分钟

### T1.2 SOCKS5 域名解析 `[E]`
- **步骤**：`curl -x socks5h://127.0.0.1:1080 http://httpbin.org/get`
- **断言（网络）**：HTTP 200（域名由 Edge 端解析）

### T1.3 HTTPS 透传 `[E]`
- **步骤**：`curl -x socks5h://127.0.0.1:1080 https://httpbin.org/get`
- **断言（网络）**：HTTP 200，TLS 握手成功

### T1.4 POST 数据转发 `[E]`
- **步骤**：`curl -x socks5h://... http://httpbin.org/post -d "key=value"`
- **断言（网络）**：body 中 `form.key == "value"`

### T1.5 大文件下载 `[E]`
- **步骤**：`curl -x socks5h://... http://speedtest.tele2.net/1MB.zip -o /dev/null -w '%{size_download}'`
- **断言（网络）**：下载字节 == 1048576

### T1.6 无 Edge 时无法出网 `[E]`
- **前置**：停止 Edge
- **步骤**：`curl -x socks5h://127.0.0.1:1080 http://httpbin.org/ip`
- **断言（网络）**：HTTP 000（SOCKS5 错误）
- **断言（DB）**：`traffic_stats` 无新增

---

## T2. 转发 — 连接生命周期

### T2.1 多请求复用同一 yamux session `[E]`
- **前置**：`DELETE FROM traffic_stats`
- **步骤**：连续 5 次 curl 请求
- **断言（日志）**：Bridge 只出现 1 次 `edge ... connected`
- **断言（DB）**：`traffic_stats COUNT(*)=5`，`edge_registrations` 仍 1 条

### T2.2 空闲 60 秒后 keepalive 维持连接 `[E]`
- **步骤**：建隧道 → 空闲 60 秒 → 再请求
- **断言（网络）**：请求成功（无需重连）

### T2.3 并发 stream 互不阻塞 `[E]`
- **步骤**：同时请求 `/delay/5` 和 `/ip`
- **断言（网络）**：`/ip` 在 2 秒内返回

### T2.4 50 并发请求 `[S]`
- **前置**：`DELETE FROM traffic_stats`
- **步骤**：50 并发 `curl -x socks5h://... http://httpbin.org/ip`
- **断言（网络）**：全部 200，无 panic
- **断言（DB）**：`traffic_stats COUNT(*)>=45`

---

## T3. 转发 — 故障恢复

### T3.1 Bridge 崩溃后 Edge 自动重连 `[E]`
- **步骤**：`kill -9 bridge` → 重启 Bridge → 等 15 秒
- **断言（日志）**：Bridge 日志出现 `edge edge-1 connected`
- **断言（网络）**：curl 恢复成功（可能第一次失败，第二次成功）

### T3.2 Edge 崩溃并重启后恢复 `[E]`
- **步骤**：`kill -9 edge` → 重启 Edge → 等 15 秒
- **断言（网络）**：curl 成功
- **断言（DB）**：`edge_registrations.last_seen` 更新

### T3.3 Client 重启后重新绑定 `[E]`
- **步骤**：`kill -9 client` → 重启 → curl
- **断言（网络）**：curl 成功
- **断言（DB）**：`client_edge_bindings` 存在，`nodes(edge-1) status='busy'`

### T3.4 apiHub 崩溃不影响已建隧道 `[E]`
- **步骤**：`kill -9 apihub` → curl
- **断言（网络）**：curl 仍然成功（隧道已建立）
- **断言（DB）**：`traffic_stats` 不新增（apiHub 不可用）

### T3.5 目标不可达返回错误 `[E]`
- **步骤**：`curl -x socks5h://... http://192.0.2.1:12345/ --connect-timeout 10`
- **断言（网络）**：HTTP 000
- **断言（DB）**：`nodes` 状态不变（Edge 正常）

### T3.6 DNS 解析失败返回错误 `[E]`
- **步骤**：`curl -x socks5h://... http://no-exist-domain.test/`
- **断言（网络）**：HTTP 000
- **断言（日志）**：Edge 日志含 `no such host`

---

## T4. 计费 — 正常扣款

### T4.1 小请求不扣费（<1KB）`[E]`
- **前置**：`UPDATE users SET balance=10000; DELETE FROM fund_flows; DELETE FROM traffic_stats`
- **步骤**：`curl -x socks5h://... http://httpbin.org/ip`（~300 字节）→ 等 3 秒
- **断言（DB）**：
  - `traffic_stats`: 新增 1 条，`bytes_in + bytes_out < 1024`
  - `users.balance = 10000`（未变）
  - `fund_flows`: 无新增（不足 1KB 不扣费）

### T4.2 下载 10KB 正确扣费 `[E]`
- **前置**：`UPDATE users SET balance=10000; DELETE FROM fund_flows; DELETE FROM traffic_stats`
- **步骤**：`curl -x socks5h://... http://httpbin.org/bytes/10240`（10KB）→ 等 3 秒
- **断言（DB）**：
  - `traffic_stats`: `bytes_in` ∈ [10240, 10700]
  - `fund_flows`: 新增 1 条
    - `amount` ∈ [-11, -10]（(10240+~100)/1024 ≈ 10）
    - `balance` ∈ [9989, 9990]
    - `type = 'traffic'`
  - `users.balance` = `fund_flows.balance`（扣后快照一致）

### T4.3 下载 1MB 正确扣费 `[E]`
- **前置**：`UPDATE users SET balance=10000; DELETE FROM fund_flows; DELETE FROM traffic_stats`
- **步骤**：`curl -x socks5h://... http://speedtest.tele2.net/1MB.zip -o /dev/null` → 等 3 秒
- **断言（DB）**：
  - `traffic_stats`: `bytes_in` ∈ [1048576, 1049000]
  - `fund_flows`: 新增 1 条
    - `amount = -1024`（1MB = 1024KB）
    - `balance = 10000 - 1024 = 8976`
  - `users.balance = 8976`

### T4.4 POST 上传计费（bytes_out 正确）`[E]`
- **前置**：`UPDATE users SET balance=10000; DELETE FROM fund_flows; DELETE FROM traffic_stats`
- **步骤**：`curl -x socks5h://... -d "$(head -c 5000 /dev/urandom | base64)" http://httpbin.org/post` → 等 3 秒
- **断言（DB）**：
  - `traffic_stats`: `bytes_out > 5000`
  - `fund_flows`: 新增 1 条，`amount < 0`
  - `users.balance < 10000`

### T4.5 多请求累计扣费 `[E]`
- **前置**：`UPDATE users SET balance=10000; DELETE FROM fund_flows; DELETE FROM traffic_stats`
- **步骤**：5 次 `curl -x socks5h://... http://httpbin.org/bytes/1024` → 等 3 秒
- **断言（DB）**：
  - `traffic_stats COUNT(*) = 5`
  - `fund_flows COUNT(*) = 5`（每次独立扣费）
  - `users.balance < 10000`（累计扣了 ≈ 5-6 KB）
  - 最后一条 `fund_flows.balance` = `users.balance`

### T4.6 并发请求计费准确 `[S]`
- **前置**：`UPDATE users SET balance=100000; DELETE FROM fund_flows; DELETE FROM traffic_stats`
- **步骤**：10 并发 `curl -x socks5h://... http://httpbin.org/bytes/1024` → 等 5 秒
- **断言（DB）**：
  - `traffic_stats COUNT(*) = 10`
  - `fund_flows COUNT(*) = 10`
  - `SUM(fund_flows.amount)` ≈ `users.balance - 100000`（总扣款一致）
  - 每条 `fund_flows.balance >= 0`（无负数）

---

## T5. 计费 — 余额控制

### T5.1 余额=0 拒绝转发 `[E]`
- **前置**：`UPDATE users SET balance=0`
- **步骤**：`curl -x socks5h://127.0.0.1:1080 http://httpbin.org/ip`
- **断言（网络）**：HTTP 000（连接失败）
- **断言（DB）**：
  - `traffic_stats`: 无新增
  - `fund_flows`: 无新增
  - `users.balance = 0`（未变）

### T5.2 余额恰好够一次请求 `[E]`
- **前置**：`UPDATE users SET balance=1; DELETE FROM fund_flows; DELETE FROM traffic_stats`
- **步骤**：`curl -x socks5h://... http://httpbin.org/bytes/1024`（≈1KB）→ 等 3 秒
- **断言（网络）**：HTTP 200（余额>0 放行）
- **断言（DB）**：
  - `users.balance = 0`（1 - 1 = 0）
  - `fund_flows`: 1 条，`amount = -1`, `balance = 0`

### T5.3 余额耗尽后后续请求被拒 `[E]`
- **前置**：`UPDATE users SET balance=2; DELETE FROM fund_flows; DELETE FROM traffic_stats`
- **步骤**：
  1. `curl -x socks5h://... http://httpbin.org/bytes/2048`（≈2KB）→ 等 3 秒
  2. `curl -x socks5h://... http://httpbin.org/ip`
- **断言**：
  - 第 1 次：HTTP 200
  - 第 2 次：HTTP 000（余额已耗尽）
  - `users.balance = 0`
  - `traffic_stats COUNT(*) = 1`（只有第 1 次）

### T5.4 充值后恢复服务 `[E]`
- **前置**：`UPDATE users SET balance=0`
- **步骤**：
  1. `curl -x socks5h://... http://httpbin.org/ip` → 失败
  2. `UPDATE users SET balance=10000`
  3. `curl -x socks5h://... http://httpbin.org/ip` → 成功
- **断言（网络）**：第 1 次 000，第 2 次 200

### T5.5 余额不足扣至 0 不为负 `[E]`
- **前置**：`UPDATE users SET balance=1; DELETE FROM fund_flows`
- **步骤**：`curl -x socks5h://... http://httpbin.org/bytes/10240`（10KB，费用≈10）→ 等 3 秒
- **断言（DB）**：
  - `users.balance = 0`（不为负数）
  - `fund_flows.balance = 0`（快照也为 0）

---

## T6. 计费 — API 接口

### T6.1 /api/traffic/report 直接调用 `[I]`
- **前置**：`DELETE FROM traffic_stats; DELETE FROM fund_flows; UPDATE users SET balance=10000`
- **步骤**：`curl -X POST http://localhost:8082/api/traffic/report -d '{"user_id":4,"edge_id":"edge-1","bytes_in":5120,"bytes_out":1024}'`
- **断言（网络）**：`{"status":"ok"}`
- **断言（DB）**：
  - `traffic_stats`: 1 条，`bytes_in=5120`, `bytes_out=1024`
  - `fund_flows`: 1 条，`amount = -6`（6144/1024=6）
  - `users.balance = 9994`

### T6.2 /api/traffic/report 参数校验 `[I]`
- **步骤**：`curl -X POST ... -d '{"user_id":0,"edge_id":"edge-1","bytes_in":100,"bytes_out":100}'`
- **断言（网络）**：HTTP 400, `user_id and edge_id required`
- **断言（DB）**：无新增

### T6.3 /api/user/balance 查询 `[I]`
- **前置**：`UPDATE users SET balance=12345`
- **步骤**：
  1. `curl http://localhost:8082/api/user/balance?user_id=4`
  2. `curl http://localhost:8082/api/user/balance?token=e2e-token`
- **断言（网络）**：两次均返回 `{"balance":12345}`

### T6.4 /api/traffic/report 无鉴权（安全漏洞）`[I]`
- **步骤**：外部直接 POST 伪造数据
- **断言**：当前返回 200（已知漏洞，待修复）

---

## T7. 计费 — 精度与边界

### T7.1 bytes_in/bytes_out 方向正确 `[E]`
- **前置**：`DELETE FROM traffic_stats`
- **步骤**：`curl -x socks5h://... http://httpbin.org/bytes/10240`（下载 10KB）→ 等 3 秒
- **断言（DB）**：
  - `bytes_in` ∈ [10240, 10700]（下载=入）
  - `bytes_out` ∈ [50, 200]（请求头=出）
  - `bytes_in > bytes_out * 10`

### T7.2 大文件下载统计精度 `[E]`
- **前置**：`DELETE FROM traffic_stats`
- **步骤**：`curl -x socks5h://... http://speedtest.tele2.net/1MB.zip -o /dev/null` → 等 3 秒
- **断言（DB）**：
  - `bytes_in` ∈ [1048576, 1049000]
  - 精度：`ABS(bytes_in - 1048576) / 1048576 < 0.01`

### T7.3 请求中断的部分流量也记录 `[E]`
- **前置**：`DELETE FROM traffic_stats`
- **步骤**：`curl -x socks5h://... http://speedtest.tele2.net/1MB.zip -m 1`（1秒后中断）→ 等 3 秒
- **断言（DB）**：`traffic_stats`: 1 条，`bytes_in > 0`

### T7.4 204 No Content 也记录流量 `[E]`
- **前置**：`DELETE FROM traffic_stats`
- **步骤**：`curl -x socks5h://... http://httpbin.org/status/204` → 等 3 秒
- **断言（DB）**：`traffic_stats`: 1 条（仅 HTTP 头流量）

---

## T8. 状态管理

### T8.1 Edge 注册后 idle `[E]`
- **断言（DB）**：`nodes(edge-1) status='idle'` + `edge_registrations` 存在

### T8.2 Client 绑定后 Edge 变 busy `[E]`
- **断言（DB）**：heartbeat 后 `nodes(edge-1) status='busy'`，`client_edge_bindings` 存在

### T8.3 心跳不覆盖 busy `[E]`
- **步骤**：等 35 秒（Edge 心跳周期）
- **断言（DB）**：`nodes(edge-1) status='busy'`（未被心跳改为 idle）

### T8.4 Bridge 滚动升级 `[E]`
- **步骤**：停 Bridge → 重编译 → 重启 → 等 15 秒
- **断言（网络）**：Edge 自动重连，Client 自动重建，curl 成功

---

## DB 断言通用 SQL

```sql
-- 节点状态
SELECT node_id, status, country, user_id,
       (last_seen > NOW() - INTERVAL '2 minutes') AS recent
FROM nodes ORDER BY id;

-- 绑定
SELECT client_id, edge_id FROM client_edge_bindings;

-- 流量
SELECT id, user_id, edge_id, bytes_in, bytes_out,
       (bytes_in+bytes_out)/1024 AS cost_kb,
       (reported_at > NOW() - INTERVAL '10 seconds') AS recent
FROM traffic_stats ORDER BY id DESC LIMIT 10;

-- 用户余额
SELECT id, username, balance FROM users;

-- 资金流水
SELECT id, user_id, amount, balance, type FROM fund_flows ORDER BY id DESC;
```

---

## 测试矩阵

| 类别 | 用例数 | DB 断言覆盖 |
|------|--------|-------------|
| T1 转发正常流程 | 6 | traffic_stats, nodes |
| T2 连接生命周期 | 4 | traffic_stats, edge_registrations |
| T3 故障恢复 | 6 | nodes, bindings, edge_registrations |
| T4 计费正常扣款 | 6 | **users.balance**, **fund_flows**, traffic_stats |
| T5 余额控制 | 5 | **users.balance**, fund_flows, traffic_stats |
| T6 计费 API | 4 | traffic_stats, fund_flows, users |
| T7 精度与边界 | 4 | traffic_stats(bytes精度) |
| T8 状态管理 | 4 | nodes, bindings, edge_registrations |
| **总计** | **39** | |

---

*Last updated: 2026-03-13*
