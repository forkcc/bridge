# proxy-bridge 重新开发 — 子任务清单

按子任务提交，每完成一项即 `git commit`。**完成某任务后请将对应项改为 `[x]` 并保存本文件。**

---

## Phase 1: 项目骨架（7）

- [x] **c1** — commit: go mod init + .gitignore
- [x] **c2** — commit: pkg/models 全部 GORM 模型定义
- [x] **c3** — commit: pkg/database GORM 连接与 AutoMigrate
- [x] **c4** — commit: configs 各组件 YAML 模板
- [x] **c5** — commit: AGENTS.md 增加 ORM 规范
- [x] **c6** — commit: pkg/config 统一 YAML 配置加载
- [x] **c7** — commit: pkg/common 公共工具与 pkg/auth Token 校验

## Phase 2: apiHub 中央服务（9）

- [x] **c8** — commit: cmd/apihub main + HTTP 框架与路由
- [x] **c9** — commit: apiHub Bridge 注册/心跳 API
- [x] **c10** — commit: apiHub Edge 注册/心跳 API
- [x] **c11** — commit: apiHub Client 认证与 Edge 列表 API
- [x] **c12** — commit: apiHub 流量上报与同步计费逻辑
- [x] **c13** — commit: apiHub 用户登录与 /health
- [x] **c14** — commit: pkg/rabbitmq 封装
- [x] **c15** — commit: apiHub 流量走 MQ + 冻结余额 + 消息跟踪
- [x] **c16** — commit: apiHub 定时任务收集冻结余额

## Phase 3: Bridge 转发服务（6）

- [x] **c17** — commit: pkg/tunnel 隧道协议 TLS+yamux+帧格式
- [x] **c18** — 已取消（方案A：不压缩）
- [ ] **c19** — commit: cmd/bridge main + 向 apiHub 注册与心跳
- [ ] **c20** — commit: bridge HTTP API health/Edge列表/Client认证代理
- [ ] **c21** — commit: bridge Edge/Client 连接管理与双向转发
- [ ] **c22** — commit: bridge 流量统计与上报 apiHub

## Phase 4: Edge 出口节点（3）

- [ ] **c23** — commit: cmd/edge main + --token/--id + 向 apiHub 注册与心跳
- [ ] **c24** — commit: edge TCP 隧道监听与 CONNECT 协议处理
- [ ] **c25** — commit: edge 连接管理器与 Android 配置 + build 脚本

## Phase 5: Client 用户端（3）

- [ ] **c26** — commit: cmd/client main + SOCKS5 服务器 CONNECT
- [ ] **c27** — commit: client Bridge 认证与 Edge 选择与隧道创建
- [ ] **c28** — commit: client 隧道转发 CONNECT->OK->双向与心跳重连

## Phase 6: 测试与文档（3）

- [ ] **c29** — commit: scripts/test-e2e.sh 端到端测试
- [ ] **c30** — commit: proto/frame.md 与 AGENTS.md 最终更新
- [ ] **c31** — commit: 单元测试 tunnel/models/edge/client

---

*共 31 项。完成一项 → 勾选 `[x]` → git commit。*
