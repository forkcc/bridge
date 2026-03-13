# proxy-bridge 项目 AI 代理指南

本文档为在 proxy-bridge Go 项目中工作的 AI 代理提供指南。涵盖构建命令、代码风格、测试和仓库规范。

## 项目概述

proxy-bridge 是一个用 Go 编写的 SOCKS5 代理集群。包含四个主要二进制文件：

- **router**：认证、bridge 列表、edge 注册与选择
- **bridge**：在客户端与 edge 之间进行隧道转发，支持 bridge 间通信
- **client**：用户端 SOCKS5 代理（监听 :1080）
- **edge**：出口节点，将流量转发到目标互联网主机

项目隧道：**Client/Edge 对 Bridge 优先 QUIC**，不可用时**降级为 TCP + yamux**；转发数据不额外压缩。

## 构建与开发命令

### 构建所有二进制文件
```bash
go build -o bridge ./cmd/bridge
go build -o client ./cmd/client
go build -o router ./cmd/router
go build -o edge ./cmd/edge
```

输出文件位于当前目录；现有实践是将它们放在 `build/` 目录中。

### Android 构建（仅 edge）
```bash
export NDK_ROOT=/home/nc/android-ndk-r27d   # 根据需要调整
./scripts/build-edge-android.sh arm64       # 或 "arm"
```
输出：`build/edge-android-{arch}`

### 运行单元测试
```bash
go test ./...                                # 运行所有测试
go test -v ./pkg/tunnel                      # 运行单个包的测试
go test -run TestReadFrame ./pkg/tunnel      # 运行特定测试
```

### 代码格式化与检查
```bash
go fmt ./...                                 # 格式化所有 Go 文件
go vet ./...                                 # 报告可疑结构
go mod tidy                                  # 清理依赖
```
未配置外部 linter（golangci‑lint）；依赖 `go vet` 和 `go fmt`。

### 端到端测试
```bash
./scripts/test-e2e.sh   # 新架构：apiHub → Bridge → Edge → Client，需 PostgreSQL（用户 postgres/密码 postgres）、数据库 proxy_bridge、build/ 下二进制
```
测试前需创建库：`createdb proxy_bridge`（若不存在）。

### 隧道协议
- 帧格式与 CONNECT 流程见 [proto/frame.md](proto/frame.md)。
- Client/Edge 与 Bridge 优先 QUIC，不可用时降级 TCP + yamux。

## ORM 与数据库

- **必须使用 GORM** 进行所有数据库操作，禁止手写 SQL（除非 GORM 无法满足的极端场景）。
- **模型定义**在 `pkg/models/` 包中，表结构通过 GORM 标签与 `TableName()` 指定。
- **建表**：使用 `pkg/database` 的 `Open()` 时自动执行 `AutoMigrate`，不维护手写 SQL 迁移文件。
- **不要**使用或创建 `migrations/` 目录；历史若存在可删除。

## 代码风格

### 导入
导入应按以下顺序分组，组间用空行分隔：

1. 标准库包
2. 内部包（模块前缀）
3. 第三方包

### 命名
- **导出标识符**：PascalCase（`Session`, `NewSession`, `Config`）
- **未导出标识符**：camelCase（`clientConns`, `edgeConns`, `handleEdge`）
- **缩写**：保持大写（`ID`, `TCP`, `TLS`, `JSON`, `YAML`）
- **接口名**：如适用，以 `-er` 结尾（`Reader`, `Writer`），否则使用描述性名词（`Session`）

### 错误处理
- 使用 `log.Printf`（或在 `main` 中使用 `log.Fatalf`）记录错误；避免 `panic`。
- 在适当的地方返回错误；除非有意忽略，否则不要用 `_` 忽略错误。
- 使用 `defer` 进行清理（关闭连接、会话、文件）。
- 发生错误时，记录简明消息并返回/退出；不要继续使用无效状态。

### 结构体标签
使用反引号标签进行 JSON 和 YAML 序列化：
```go
type config struct {
    Listen      string `yaml:"listen"`
    RouterURL   string `yaml:"router_url"`
    NoAuth      bool   `yaml:"no_auth"`
    MyAddr      string `yaml:"my_addr"`
}
```

### 并发
- 使用 `sync.RWMutex` 保护共享数据（读多写少时优先使用 `RWMutex` 而非 `Mutex`）。
- 对简单计数器使用 `atomic` 操作（`int64`, `int32`）。
- 避免在网络 I/O 期间持有锁；在拨号或写入前释放锁。
- 尽可能使用 `defer mu.Unlock()`，但注意不要持有锁过长时间。

### 注释
- 注释使用中文（现有代码库的主要语言）。如无法使用中文，英文也可接受。
- 为导出的函数、类型和包提供简明描述。
- 谨慎使用行内注释；优先使用自解释的名称。
- 对复杂算法或非显而易见的逻辑添加注释。

## 项目结构

```
.
├── cmd/                 # 各二进制文件的主包
│   ├── bridge/
│   ├── client/
│   ├── edge/
│   └── router/
├── internal/            # 私有应用代码
│   └── bridge/         # 核心 bridge 逻辑
├── pkg/                # 公共可复用包
│   └── tunnel/         # 隧道协议实现
├── configs/            # YAML 配置文件
├── scripts/            # 构建和测试脚本
├── migrations/         # 数据库迁移（如有）
├── proto/              # 协议文档
├── docs/               # 设计文档
└── build/              # 编译生成的二进制文件
```

## 测试指南

### 单元测试
- 测试文件与测试代码放在一起（`*_test.go`）。
- 使用标准 Go testing 包；无需外部测试框架。
- 遵循 `func TestXxx(t *testing.T)` 模式。
- 对多个输入输出对使用表驱动测试。
- 适当模拟外部依赖（网络、文件系统）。

### 集成/端到端测试
- 使用 `scripts/test-e2e.sh` 作为端到端测试模板。
- 按正确顺序启动组件：router → bridge → edge → client。
- 用真实 HTTP 请求（curl）验证连通性，完成后清理进程。

### 运行单个测试
```bash
go test -v ./pkg/tunnel -run TestReadFrame
```

## Git & 提交规范

- 提交信息应简洁，描述*为什么*而非*做了什么*。
- 使用现在时、命令式风格："add tunnel compression"、"fix deadlock in bridge peer"。
- 如适用，在提交前添加范围前缀：`bridge: `、`tunnel: `、`client: `。
- 不要提交二进制文件（除非是 `build/` 目录下属于发布流程的文件）。
- 提交前确保 `go fmt` 和 `go vet` 通过。

## Cursor / Copilot Rules

未定义项目特定的 Cursor（`.cursor/rules/`）或 Copilot（`.github/copilot‑instructions.md`）规则。请遵循本文档中的指南。

## AI代理语言要求

- **模型必须用中文回复**：所有AI代理在与用户交互、解释代码、回答问题、编写文档时，必须使用中文进行回复。
- **代码注释**：代码注释应优先使用中文（如"注释"部分所述），仅在无法使用中文时使用英文。
- **技术术语**：技术术语、命令、包名、函数名等可保持英文原样。
- **例外情况**：当用户明确要求使用英文回复时，可按照用户要求切换语言。

## TODO管理规范

- **任务完成检查**：每次任务完成后，必须检查对应的TODO是否已完成，并更新其状态。
- **TODO创建**：执行任务时，如果对应的TODO尚未创建，必须立即创建相应的TODO条目。
- **TODO维护**：TODO列表只能增加新的任务项，不能删除已有的TODO条目。已完成的任务应标记为"completed"状态，而不是删除。

## Edge Android优化规范

- **优化要求**：edge是运行在Android设备上的出口节点，必须优化到极致以满足移动设备资源限制。
- **32位支持**：必须支持32位Android（ARMv7架构），确保在旧设备上兼容运行。
- **内存优化**：
  - 避免内存泄漏，使用`defer`正确释放资源
  - 限制并发连接数，防止内存耗尽
  - 使用连接池复用TCP连接
- **性能优化**：
  - 启用TCP_NODELAY减少延迟
  - 合理设置缓冲区大小，避免频繁分配
  - 使用zstd压缩时注意CPU消耗，在低端设备上可适当降低压缩级别
- **二进制大小**：
  - 使用`-ldflags="-s -w"`减小二进制体积
  - 避免引入不必要的依赖
  - 考虑使用`upx`进一步压缩（如果许可允许）
- **稳定性**：
  - 正确处理网络断开和重连
  - 实现心跳机制保持连接活跃
  - 在资源不足时优雅降级

## 发现与经验教训

### SOCKS5转发超时问题
在端到端测试中遇到SOCKS5转发超时问题，原因如下：

1. **Edge隧道响应协议**：Edge在收到CONNECT命令后，需要发送成功响应（如"OK\n"）给Client，否则Client会在`forwardViaTunnel`函数中等待响应而超时。
2. **CONNECT命令格式**：Client发送的格式为`CONNECT host:port\n`，Edge需要正确解析并处理换行符后的剩余数据。
3. **连接超时设置**：在`internal/client/socks5_server.go`的`forwardViaTunnel`函数中，设置了隧道连接读取超时为30秒，但Edge需要在5秒内响应。
4. **错误处理**：Edge连接目标服务器失败时应发送错误响应，而不是直接关闭连接。

已修复方案：
- Edge在`handleConnectRequest`函数中添加成功响应发送
- 优化超时时间为5秒
- 添加TCP_NODELAY设置减少延迟

### Token认证机制
**Token 与 User 挂钩方式**：`nodes` 表**没有** `user_id`；关联靠 **token**：`users.token` 存用户令牌，`nodes.token` 指向该值（同 token = 同用户）。

1. **用户认证**：`users` 表有用户名密码和 **`token`** 字段（用户级令牌，唯一）
2. **节点认证**：`nodes` 表的 `token` 须与 **`users.token`** 一致；认证时先查 `users.token` 存在，再按 `node_type` 从 `nodes` 取对应节点
   - Client 用 **client 类型** 节点（`nodes.token = users.token`）；拉取 Edge 列表只返回 **同一 token** 的 edge
   - Edge 用 **edge 类型** 节点（同上）
   - 若 `users` 中无该 token，则无法连接

**重要**：Edge 启动须传 `--token` 和 `--id`。为用户在 `users` 表设置 `token`，并在 `nodes` 表创建 client/edge 节点且 `nodes.token` 与 `users.token` 相同。

### 隧道复用优化
Edge的`basicTunnel.Process`已优化支持单个TCP连接处理多个CONNECT请求：
- 主循环持续读取CONNECT命令
- 每个请求创建独立的转发协程
- 使用缓冲池（32KB）减少内存分配
- 区分正常EOF和错误，避免将连接关闭记录为错误

### 端口占用问题
测试过程中常见问题：
- SOCKS5端口1080被占用：`pkill -f "client --config"` 清理旧进程
- Edge隧道端口60001被占用：`lsof -ti:60001 | xargs kill -9`
- 建议使用`scripts/test-e2e.sh`脚本管理进程生命周期

### Edge 一启动就退出
原因：`Run()` 中只调用一次 `runTunnel()`，隧道断开（Bridge 未起、网络断开、Bridge 重启等）后函数返回，进程随即退出。  
处理：在 `Run()` 中对 `runTunnel()` 做死循环 + 断线后 5 秒重连，使 Edge 常驻并在隧道恢复后自动重连。

### 如何确认流量从 Edge 流出
- **看 Edge 日志**：每次经本 Edge 出口的 CONNECT 会打一行 `edge: connect <host:port> (流量经本 edge 出口)`，有该日志即说明请求从该 Edge 流出。
- **看出口 IP**：通过 SOCKS5 访问“出口 IP”类服务，返回的 IP 应为 **Edge 所在机器**的公网 IP，而非本机。示例：
  - `curl -x socks5h://127.0.0.1:1080 https://api.ipify.org`
  - `curl -x socks5h://127.0.0.1:1080 https://ifconfig.me`
  若返回的 IP 与运行 Edge 的机器公网 IP 一致，即可确认流量是从该 Edge 流出的。

### 架构演进
从旧router架构迁移到apiHub架构：
- apiHub作为中央API服务，负责认证、计费、注册、状态管理
- Bridge只负责流量转发，启动后向apiHub注册
- Client和Edge通过apiHub认证，Client可以选择国家后随机绑定该国家edge
- clients和edges合并为统一的nodes表，用node_type字段区分

### 性能要求
- **Bridge集群间通信**：不能有高延迟，Bridge转发以短连接为主
- **Edge Android优化**：必须支持32位ARMv7，内存限制50MB，低功耗模式
- **连接管理**：Client和Edge都只与Bridge保持1个TCP连接，通过多路复用转发流量
- **数据流模式**：请求走 Client→Bridge→Edge→目标，响应走 目标→Edge→Bridge→Client；不允许 Client 与 Edge 直连，所有流量经 Bridge 中转（见 `proto/frame.md`）

### Client 与 Edge 按国家匹配
- **Edge 国家**：Edge 不配置国家；由 **Bridge 在 Edge 连上隧道时**根据对端 IP 上报 apiHub（`POST /api/edge/country`）。若对端为 `127.0.0.1`/`::1`/`localhost` 则默认上报 `cn`，便于本地测试。
- **Client 国家**：Client 启动时**必须指定国家**（`--country cn` 或配置 `country`），拉取 Edge 列表时带 `?country=cn`，apiHub 只返回该国且在线的 edge，实现按国家匹配。

### Client 与 Edge 双向解绑
- **绑定**：Client 心跳时上报当前 `edge_id`，apiHub 维护 `client_edge_bindings` 表（client_id=token, edge_id）。
- **仅返回在线 Edge**：拉取 Edge 列表时只返回 `last_seen` 在 2 分钟内的 edge，避免选到已下线的节点。
- **Client 下线 → Edge 解绑**：apiHub 定时任务（约 1 分钟）删除“client 超过 2 分钟未心跳”的绑定，该 edge 可被其他 client 使用。
- **Edge 下线 → Client 换绑**：定时任务删除“edge 超过 2 分钟未心跳”的绑定；Client 侧隧道失败会 `clearTunnel()`，下次请求重新拉列表并绑定新的在线 edge。

## 附加说明

- 项目使用 Go 1.24.0（见 `go.mod`）。
- 依赖项极少；除非绝对必要，避免添加新依赖。
- 配置基于 YAML；将配置文件放在 `configs/` 中，并从根目录引用它们。
- 隧道协议定义在 `proto/frame.md` 中；修改帧格式时请参考该文档。
- Android 交叉编译会设置 Go 环境中的 `CC`、`CXX`、`GOOS`、`GOARCH`；如果正常构建失败，运行 `go env -u CC CXX GOOS GOARCH` 清除它们。

---

*Last updated: Fri Mar 13 2026*