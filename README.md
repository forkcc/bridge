# proxy-bridge

SOCKS5 代理集群，由四个组件协同工作，支持多出口节点、按国家选择线路、实时流量计费。

## 架构

```
用户应用 → Client(:1080) → Bridge → Edge → 目标网站
               ↕               ↕
            apiHub (认证/计费/注册)
```

| 组件 | 职责 |
|------|------|
| **apiHub** | 中央 API 服务：用户认证、节点注册、余额管理、流量计费 |
| **Bridge** | 中继转发：在 Client 与 Edge 之间建立隧道，IP 归属判断，余额拦截 |
| **Client** | 用户端 SOCKS5 代理（监听 `:1080`），连接 Bridge 隧道转发流量 |
| **Edge** | 出口节点：接收 Bridge 转发的请求，连接目标服务器。支持 Android 部署 |

## 技术特点

- **自定义隧道协议**：TCP + yamux 多路复用，单连接承载所有流量
- **按国家选路**：Bridge 通过 ip2region 识别 Edge 所在国家，Client 按偏好选择出口
- **实时计费**：按 KB 粒度直接扣款，Bridge 本地缓存余额（TTL 60s）支撑高并发拦截
- **Android 优化**：Edge 支持 ARMv7/ARM64 交叉编译，内存限制 50MB，低功耗模式
- **TUI 界面**：Client 提供终端实时显示连接状态和网速

## 快速开始

### 前置条件

- Go 1.24+
- PostgreSQL

### 构建

```bash
go build -o build/apihub ./cmd/apihub
go build -o build/bridge ./cmd/bridge
go build -o build/client ./cmd/client
go build -o build/edge   ./cmd/edge
```

### 初始化数据库

```bash
go run ./cmd/seed
```

### 启动（按顺序）

```bash
./build/apihub configs/apihub.yaml
./build/bridge configs/bridge.yaml
./build/edge   --token edge-token --id edge-1 configs/edge.yaml
./build/client --token client-token configs/client.yaml
```

启动后通过 `localhost:1080` 使用 SOCKS5 代理：

```bash
curl -x socks5h://localhost:1080 https://httpbin.org/ip
```

### Android 构建（Edge）

```bash
export NDK_ROOT=/path/to/android-ndk
./scripts/build-edge-android.sh arm64  # 或 arm
```

## 配置文件

所有配置位于 `configs/` 目录，YAML 格式：

| 文件 | 说明 |
|------|------|
| `apihub.yaml` | 监听地址、PostgreSQL 连接 |
| `bridge.yaml` | 监听地址、隧道端口、apiHub 地址 |
| `client.yaml` | SOCKS5 端口、Bridge 地址、Token、国家偏好 |
| `edge.yaml` | 隧道端口、apiHub 地址、连接限制、Android 优化参数 |

## 项目结构

```
cmd/            各组件入口
internal/       私有业务逻辑
  apihub/       API 服务（认证、计费、注册）
  bridge/       中继转发
  client/       SOCKS5 + TUI
  edge/         出口节点
pkg/            可复用公共包
  tunnel/       yamux 隧道封装
  models/       GORM 数据模型
  database/     数据库连接
  auth/         Token 认证
configs/        配置文件
scripts/        构建与测试脚本
proto/          协议文档
```

## 测试

```bash
go test ./...                          # 单元测试
./scripts/test-e2e.sh                  # 端到端测试（单 Bridge）
./scripts/test-two-bridges.sh          # 多 Bridge 场景
```

## License

Private
