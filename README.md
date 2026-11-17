# ggskit

基于 [gactor](https://github.com/godyy/gactor) 和 [gcluster](https://github.com/godyy/gcluster) 实现的游戏服务器应用开发工具包.

`ggskit` 面向分布式游戏服务器场景, 在 `gactor`(Actor 模型) 与 `gcluster`(集群通信) 之上提供了一套可直接复用的基础能力与基础设施封装, 涵盖协议编解码、集群节点发现、Actor 注册与寻址、数据持久化、加解密与鉴权、日志、监控探针等模块.

## 目录结构

```
ggskit
├── base/            基础能力
│   ├── auth/jwt     JWT 鉴权
│   ├── codec        协议编解码 (c2s / s2s)
│   ├── config       TOML 配置加载
│   ├── crypto       加解密与签名 (aes / rsa / pem)
│   ├── db           数据库客户端 (mongo / redis)
│   ├── env          环境管理 (dev / prod / debug)
│   ├── errs         带错误码的错误
│   ├── flags        统一 flag 注册与解析
│   ├── logger       日志
│   └── protocol     protobuf 协议注册表
├── infra/           基础设施
│   ├── actor        基于 gactor 的 Actor 封装
│   ├── cluster      基于 gcluster + etcd 的集群服务
│   ├── mongobd      MongoDB 操作后台
│   ├── monitor      监控 (pprof / 探针)
│   └── noderouter   节点路由选择器
└── utils/           通用工具
```

## 模块说明

### base —— 基础能力

| 模块 | 路径 | 说明 |
| --- | --- | --- |
| 协议注册 | [protocol](base/protocol) | protobuf 消息注册表, 依据消息全名 FNV 哈希生成 `PID`, 支持注册、查 PID、按 PID 创建实例、校验 PID 与类型 |
| 编解码 | [codec/c2s](base/codec/c2s) · [codec/s2s](base/codec/s2s) | C2S 数据包(包头 `Pt`/`Seq`/`PID` + proto 负载)与 S2S 负载(4 字节 PID + proto 负载)的编解码 |
| 加解密 | [crypto](base/crypto) | 通用 `Encryptor`/`Decryptor`/`Signer`/`Verifier` 接口; [aes](base/crypto/aes) 提供 AES-GCM/CBC, [rsa](base/crypto/rsa) 提供 PKCS#1 v1.5 分块加解密与 PSS 签名验签, 并含 PEM 读写 |
| 鉴权 | [auth/jwt](base/auth/jwt) | 基于 EdDSA 的 JWT 签发与解析, 支持从 PEM 加载公私钥 |
| 配置 | [config](base/config) | 加载 TOML 配置文件, 支持通过 `ConfigWithFlags` 在解码后应用 flag 覆盖 |
| flag | [flags](base/flags) | 统一的 flag 注册、解析与解析完成回调 |
| 环境 | [env](base/env) | 环境(`dev`/`prod`)与调试模式管理, 内置 `--env-stage`、`--env-debug` flag |
| 错误 | [errs](base/errs) | 携带错误码的 `Error` 接口及 `WithErrCode*` 构造方法 |
| 日志 | [logger](base/logger) | 基于 `glog` 的日志创建, 支持标准输出与文件输出 |
| 数据库 | [db/mongo](base/db/mongo) · [db/redis](base/db/redis) | MongoDB / Redis 客户端连接封装; redis 另提供分布式锁 `DLock` |

### infra —— 基础设施

- **[actor](infra/actor)**: 对 `gactor` 的工程化封装, 提供:
  - `Service` / `Client`: Actor 服务与客户端, 内置同步/异步 RPC、Cast、Forward 等能力;
  - `ProtoRegistry` / `Codec` / `C2SPayload` / `S2SPayload`: C2S 与 S2S 协议注册与编解码;
  - `ContextSugarUtil` / `ActorSugarUtil`: 上下文与 Actor 的 RPC 语法糖;
  - `Model` / `Modules` / `ModuleRegistry`: 基于 BSON 的数据模型与模块化持久化;
  - `Registry` / `ServerStore`: 基于 Redis 的 Actor 位置注册表(含变更 Stream 与本地缓存)与所属服务器存储;
  - `Router`: 结合节点分组与 rendezvous hashing 的 Actor 部署节点选择。
- **[cluster](infra/cluster)**: 基于 `gcluster` + etcd 的集群服务。`Center` 负责在 etcd 中注册自身、同步并监听节点列表变更(带租约保活与退避重试); `Node` 描述节点信息; `Service` 组合 Center 与 Agent, 提供节点收发与查询。
- **[mongobd](infra/mongobd)**: MongoDB 异步操作后台。`BD` 按哈希键将操作符(`OpLoad`/`OpUpdate`/`OpBulk`)分发到多个 `worker` 执行, 支持并发控制、默认超时与 panic 保护。
- **[monitor](infra/monitor)**: 监控能力。`pprof` 注册标准 pprof 路由; `probe` 提供 `/healthz` 与 `/readyz` 健康就绪探针(支持就绪检查器、门闩与缓存策略)。
- **[noderouter](infra/noderouter)**: 节点路由选择器 `Selector` 接口及基于 rendezvous hashing 的 `RendezvousSelector` 实现, 支持分组、全量/增量更新与多候选选取。

### utils —— 通用工具

- **[ctxutils](utils/ctxutils)**: 封装 `context.WithTimeout`, 在调试模式下自动放宽超时。
- **[httputils](utils/httputils)**: HTTP 请求封装, 支持请求选项与响应检查器, 并提供 `PostJson` / `GetJson` 便捷方法。
- **[ip.go](utils/ip.go)**: 解析本机可路由的 IPv4 地址。
- **[shutdown.go](utils/shutdown.go)**: 监听 `SIGINT` / `SIGTERM` 关闭信号。

## 依赖

核心依赖:

- [gactor](https://github.com/godyy/gactor) —— Actor 模型
- [gcluster](https://github.com/godyy/gcluster) —— 集群通信
- [glog](https://github.com/godyy/glog) —— 日志
- [grendezvous](https://github.com/godyy/grendezvous) —— rendezvous hashing
- etcd client / go-redis / mongo-driver —— 集群与存储
- protobuf —— 协议编解码

## 环境要求

- Go 1.25+

## 许可证

本项目为内部工具包, 许可证以仓库约定为准.
