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
| `traffic_stats` | `id`, `user_id`, `edge_id`, `bytes_in`, `bytes_out`, `reported_at` | 流量审计（每连接） |
| `fund_flows` | `id`, `user_id`, `amount`, `balance`, `type` | 资金流水，amount 单位 KB |

## 计费规则

### 核心模型

- **余额单位**：KB（`users.balance`）
- **结算方式**：立即结算，每次连接结束后即扣费/赚取
- **换算公式**：`totalKB = (bytes_in + bytes_out) / 1024`
- **最小计量**：不满 1KB 的流量不计费（整除取 floor）
- **无价格系数**：1KB 流量 = 1KB 余额

### 结算流程

1. Bridge 转发完毕后调用 `/api/traffic/report`
2. apiHub 在同一事务中完成：审计记录 + Client 扣费 + Edge 赚取
3. Client 扣费：`GREATEST(balance - totalKB, 0)`（不为负）
4. Edge 赚取：`balance + totalKB`

### 资金流水

- **fund_flows.amount**：KB 数（Client 为负，Edge 为正）
- **fund_flows.balance**：变动后的 `users.balance` 快照（KB）
- **fund_flows.type**：`traffic_client`（花费）/ `traffic_edge`（赚取）

### 余额检查

- Bridge 每次转发前查询余额（本地缓存 TTL 60s），`balance <= 0` 时拒绝
- 扣费后自动更新本地缓存

### API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/traffic/report` | POST | 审计记录 + 立即结算（Client 扣费 + Edge 赚取） |
| `/api/user/balance` | GET | 查询用户余额 |

## 初始状态（seed 后）

```sql
-- users: balance=10000 (KB)
-- nodes: client(online), edge(idle), user_id 已关联
-- 其他表均为空
```

---

## T1. 转发 — 正常流程

### T1.1 全链路 SOCKS5 转发 `[E]`
- **步骤**：`curl -x socks5h://127.0.0.1:1080 http://httpbin.org/ip`
- **断言（网络）**：HTTP 200，body 含 `origin`
- **断言（DB）**：
  - `traffic_stats`: 新增 1 条，`user_id>0`, `edge_id='edge-1'`
  - `fund_flows`: 无新增（流量 <1KB，totalKB=0）
  - `users.balance` 不变

### T1.2 SOCKS5 域名解析 `[E]`
- **步骤**：`curl -x socks5h://127.0.0.1:1080 http://httpbin.org/get`
- **断言（网络）**：HTTP 200

### T1.3 HTTPS 透传 `[E]`
- **步骤**：`curl -x socks5h://127.0.0.1:1080 https://httpbin.org/get`
- **断言（网络）**：HTTP 200

### T1.4 POST 数据转发 `[E]`
- **步骤**：`curl -x socks5h://... http://httpbin.org/post -d "key=value"`
- **断言（网络）**：body 中 `form.key == "value"`

### T1.5 大文件下载 + 立即结算 `[E]`
- **前置**：`DELETE FROM fund_flows; DELETE FROM traffic_stats; UPDATE users SET balance=1048576`
- **步骤**：`curl -x socks5h://... http://httpbin.org/bytes/102400 -o /dev/null`
- **断言（网络）**：下载 == 102400
- **断言（DB）**：
  - `traffic_stats`: bytes_in ≈ 102400
  - `fund_flows`: 2 条，`traffic_client` amount ≈ -100，`traffic_edge` amount ≈ +100
  - `users.balance` 变化 = 0（同用户，client 扣 = edge 赚）

### T1.6 无 Edge 时无法出网 `[E]`
- **前置**：停止 Edge
- **步骤**：`curl -x socks5h://127.0.0.1:1080 http://httpbin.org/ip`
- **断言（网络）**：HTTP 000

---

## T2. 转发 — 连接生命周期

### T2.1 多请求复用同一 yamux session `[E]`
- **前置**：`DELETE FROM traffic_stats`
- **步骤**：连续 5 次 curl 请求
- **断言（DB）**：`traffic_stats COUNT(*)=5`，`edge_registrations` 仍 1 条

### T2.2 空闲 60 秒后 keepalive 维持连接 `[E]`
- **步骤**：建隧道 → 空闲 60 秒 → 再请求
- **断言（网络）**：请求成功

### T2.3 并发 stream 互不阻塞 `[E]`
- **步骤**：同时请求 `/delay/5` 和 `/ip`
- **断言（网络）**：`/ip` 在 2 秒内返回

### T2.4 50 并发请求 `[S]`
- **前置**：`DELETE FROM traffic_stats`
- **步骤**：50 并发 `curl -x socks5h://... http://httpbin.org/ip`
- **断言（网络）**：全部 200
- **断言（DB）**：`traffic_stats COUNT(*)=50`

---

## T3. 转发 — 故障恢复

### T3.1 Bridge 崩溃后 Edge 自动重连 `[E]`
- **步骤**：`kill -9 bridge` → 重启 → 等 15 秒
- **断言（日志）**：Bridge 日志 `edge edge-1 connected`
- **断言（网络）**：curl 成功

### T3.2 Edge 崩溃并重启后恢复 `[E]`
- **步骤**：`kill -9 edge` → 重启 → 等 15 秒
- **断言（网络）**：curl 成功
- **断言（DB）**：`edge_registrations.last_seen` 更新

### T3.3 Client 重启后重新绑定 `[E]`
- **步骤**：`kill -9 client` → 重启 → curl
- **断言（网络）**：curl 成功
- **断言（DB）**：`client_edge_bindings` 存在

### T3.4 apiHub 崩溃不影响已建隧道 `[E]`
- **步骤**：`kill -9 apihub` → curl
- **断言（网络）**：curl 成功（隧道已建立）

### T3.5 目标不可达返回错误 `[E]`
- **步骤**：`curl -x socks5h://... http://192.0.2.1:12345/ --connect-timeout 10`
- **断言（网络）**：HTTP 000

### T3.6 DNS 解析失败返回错误 `[E]`
- **步骤**：`curl -x socks5h://... http://no-exist-domain.test/`
- **断言（网络）**：HTTP 000

---

## T4. 计费 — 立即结算

### T4.1 小请求不扣费（<1KB） `[E]`
- **前置**：`UPDATE users SET balance=1048576; DELETE FROM fund_flows; DELETE FROM traffic_stats`
- **步骤**：`curl -x socks5h://... http://httpbin.org/ip`（~300 字节）
- **断言（DB）**：
  - `traffic_stats`: 1 条
  - `fund_flows`: 无新增（totalKB=0）
  - `users.balance = 1048576`

### T4.2 大文件下载立即扣费 `[E]`
- **前置**：`UPDATE users SET balance=1048576; DELETE FROM fund_flows; DELETE FROM traffic_stats`
- **步骤**：`curl -x socks5h://... http://httpbin.org/bytes/102400`
- **断言（DB）**：
  - `fund_flows`: 2 条，`traffic_client` amount ≈ -100，`traffic_edge` amount ≈ +100
  - 结算立即发生（无需等 1GB 累积）

### T4.3 /api/traffic/report 直接调用 `[I]`
- **前置**：`UPDATE users SET balance=1048576; DELETE FROM fund_flows; DELETE FROM traffic_stats`
- **步骤**：`curl -X POST .../api/traffic/report -d '{"user_id":4,"edge_id":"edge-1","bytes_in":1048576,"bytes_out":1024}'`
- **断言（DB）**：
  - `traffic_stats`: 1 条，`bytes_in=1048576`, `bytes_out=1024`
  - `fund_flows`: 2 条，amount = ±1025（(1048576+1024)/1024）
  - `users.balance` 变化 = 0（同用户抵消）

### T4.4 /api/traffic/report 参数校验 `[I]`
- **步骤**：`curl -X POST ... -d '{"user_id":0,"edge_id":"edge-1","bytes_in":100,"bytes_out":100}'`
- **断言（网络）**：HTTP 400

### T4.5 并发结算原子性 `[S]`
- **前置**：`UPDATE users SET balance=10485760; DELETE FROM fund_flows`
- **步骤**：10 并发 `curl -X POST .../api/traffic/report -d '{"user_id":4,"edge_id":"edge-1","bytes_in":1048576,"bytes_out":0}'`
- **断言（DB）**：
  - `fund_flows COUNT(*) = 20`（10 × client + edge）
  - `SUM(amount WHERE type='traffic_client') = -10240`
  - `SUM(amount WHERE type='traffic_edge') = 10240`

---

## T5. 计费 — 余额控制

### T5.1 余额=0 拒绝转发 `[E]`
- **前置**：`UPDATE users SET balance=0`，重启 Bridge（清缓存）
- **步骤**：`curl -x socks5h://127.0.0.1:1080 http://httpbin.org/ip`
- **断言（网络）**：HTTP 000
- **断言（DB）**：`traffic_stats` 无新增，`users.balance = 0`

### T5.2 余额>0 放行，余额=0 拒绝 `[E]`
- **前置**：`UPDATE users SET balance=100`，重启 Bridge
- **步骤**：
  1. curl → 成功
  2. `UPDATE users SET balance=0`
  3. 等 65 秒（缓存过期）
  4. curl → 失败
- **断言（网络）**：第 1 次 200，第 4 次 000

### T5.3 充值后恢复服务 `[E]`
- **前置**：`UPDATE users SET balance=0`，重启 Bridge
- **步骤**：
  1. curl → 失败
  2. `UPDATE users SET balance=10000`
  3. 等 65 秒（缓存过期）
  4. curl → 成功
- **断言（网络）**：第 1 次 000，第 4 次 200

### T5.4 扣费不为负数 `[I]`
- **前置**：`UPDATE users SET balance=500; DELETE FROM fund_flows`
- **步骤**：`curl -X POST .../api/traffic/report -d '{"user_id":4,"edge_id":"edge-1","bytes_in":1048576,"bytes_out":0}'`
- **断言（DB）**：
  - `fund_flows(traffic_client).balance = 0`（GREATEST(500-1024, 0) = 0）

---

## T6. 计费 — 审计与查询

### T6.1 /api/user/balance 查询 `[I]`
- **前置**：`UPDATE users SET balance=12345`
- **步骤**：
  1. `curl .../api/user/balance?user_id=4`
  2. `curl .../api/user/balance?token=e2e-token`
- **断言（网络）**：两次均返回 `{"balance":12345}`

### T6.2 bytes_in/bytes_out 方向正确 `[E]`
- **前置**：`DELETE FROM traffic_stats`
- **步骤**：`curl -x socks5h://... http://httpbin.org/bytes/10240`（下载 10KB）
- **断言（DB）**：
  - `bytes_in` ≈ 10240（下载=入）
  - `bytes_out` < 200（请求头=出）
  - `bytes_in > bytes_out * 10`

### T6.3 204 No Content 也记录流量 `[E]`
- **前置**：`DELETE FROM traffic_stats`
- **步骤**：`curl -x socks5h://... http://httpbin.org/status/204`
- **断言（DB）**：`traffic_stats`: 1 条

---

## T7. 状态管理

### T7.1 Edge 注册后 idle `[E]`
- **断言（DB）**：`nodes(edge-1) status='idle'` + `edge_registrations` 存在

### T7.2 Client 绑定后 Edge 变 busy `[E]`
- **断言（DB）**：heartbeat 后 `nodes(edge-1) status='busy'`，`client_edge_bindings` 存在

### T7.3 心跳不覆盖 busy `[E]`
- **步骤**：等 35 秒（Edge 心跳周期）
- **断言（DB）**：`nodes(edge-1) status='busy'`

### T7.4 Bridge 滚动升级 `[E]`
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

-- 流量审计
SELECT id, user_id, edge_id, bytes_in, bytes_out,
       (reported_at > NOW() - INTERVAL '10 seconds') AS recent
FROM traffic_stats ORDER BY id DESC LIMIT 10;

-- 用户余额（单位 KB）
SELECT id, username, balance FROM users;

-- 资金流水（amount 单位 KB）
SELECT id, user_id, amount, balance, type FROM fund_flows ORDER BY id DESC;
```

---

## 测试矩阵

| 类别 | 用例数 | 核心覆盖 |
|------|--------|----------|
| T1 转发正常流程 | 6 | traffic_stats, 立即结算 |
| T2 连接生命周期 | 4 | traffic_stats, edge_registrations |
| T3 故障恢复 | 6 | nodes, bindings |
| T4 立即结算 | 5 | **fund_flows(KB)**, traffic/report API, 并发原子性 |
| T5 余额控制 | 4 | **余额检查**, 缓存 TTL, 扣至 0 |
| T6 审计与查询 | 3 | balance 查询, bytes 方向 |
| T7 状态管理 | 4 | nodes, bindings, edge_registrations |
| **总计** | **32** | |

---

*Last updated: 2026-03-13*
