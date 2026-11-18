# AGENTS.md

本文件面向在本仓库中工作的 AI 编码助手与开发者, 说明 `ggskit` 的项目定位、常用命令、代码约定与各模块的开发要点.

## 项目概述

`ggskit` 是基于 [gactor](https://github.com/godyy/gactor) 与 [gcluster](https://github.com/godyy/gcluster) 的分布式游戏服务器应用开发工具包, 在 Actor 模型与集群通信之上提供可复用基础能力与基础设施封装.

- module: `github.com/godyy/ggskit`
- Go 版本: 1.25+
- 分层: `base`(基础能力) / `infra`(基础设施) / `utils`(通用工具)

## 常用命令

```bash
go build ./...            # 编译全部包
go vet ./...              # 静态检查
go test ./...             # 运行全部测试
go test ./infra/actor/... # 运行指定包测试
gofmt -l .                # 检查格式 (应无输出)
gofmt -w .                # 格式化
```

- 提交前至少保证 `go build ./...`、`go vet ./...`、`gofmt` 通过.
- 测试使用标准库 `testing`, 未引入第三方断言框架; 相关用例可能依赖 MongoDB / Redis / etcd, 缺失外部依赖时应跳过或注明.

## 目录结构

```
ggskit
├── base/            基础能力
│   ├── auth/jwt     JWT 鉴权 (EdDSA)
│   ├── codec        协议编解码 (c2s / s2s)
│   ├── config       TOML 配置加载
│   ├── crypto       加解密与签名 (aes / rsa / pem)
│   ├── db           数据库客户端 (mongo / redis, 含分布式锁)
│   ├── env          环境管理 (dev / prod / debug)
│   ├── errs         带错误码的错误
│   ├── flags        统一 flag 注册与解析
│   ├── logger       日志 (基于 glog)
│   └── protocol     protobuf 协议注册表
├── infra/           基础设施
│   ├── actor        基于 gactor 的 Actor 封装
│   ├── cluster      基于 gcluster + etcd 的集群服务
│   ├── mongobd      MongoDB 异步操作后台
│   ├── monitor      监控 (pprof / 探针 / prometheus 指标暴露)
│   └── noderouter   节点路由选择器 (rendezvous hashing)
└── utils/           通用工具 (ctxutils / httputils / ip / shutdown)
```

## 代码约定

### 命名与注释

- 注释使用中文, 简洁明了, 说明"是什么/为什么", 不复述代码本身.
- 导出标识符使用 godoc 风格: 以标识符名开头, 以句号结尾, 例如 `// Service 封装 gactor.Service.`
- 简单实现代码内部的行内注释按需添加, 不强制每个字段/每行都写注释; 公开 API、配置结构体字段、易误用的逻辑需加注释.
- 接收者命名简短且一致: `s *Service`、`env *envImpl`、`c *Center` 等.
- 私有实现类型使用 `xxxImpl` 命名 (如 `envImpl`), 对外暴露接口.
- 包名短小写, 避免下划线与驼峰.

### 错误处理 (重要)

- 需要嵌套上下文错误时, 使用 `github.com/pkg/errors`(统一别名为 `pkgerrors`) 的 `With` 系列方法, **不要**用 `fmt.Errorf` 包装:
  - `pkgerrors.WithMessage(err, "connect")`
  - `pkgerrors.WithMessagef(err, "unmarshal proto pid=%d", pid)`
  - `pkgerrors.New("rsa: empty db")`
- `fmt.Errorf` 仅用于生成无嵌套上下文的原始错误 (如参数校验失败).
- 需要携带错误码时, 使用 `base/errs` 的 `WithErrCodeMsg` / `WithErrCodeMsgf` / `WithErrCodeErr`.
- 预定义可判断的哨兵错误使用 `errors.New` (如 `redis.ErrLockNotObtained`), 便于 `errors.Is` 判断.

### 接口与实现

- 优先面向接口编程, 接口定义在**使用方**侧且保持最小 (如 `codec/c2s.ProtoRegistry`、`monitor.HttpRouter`).
- 具体实现隐藏为包内私有类型, 通过 `NewXxx` 构造函数返回接口或指针.
- 构造函数返回 `(*T, error)`, 对必填依赖做显式校验并返回可读错误.
- 结构体配置 (Config) 采用独立 `XxxConfig`, 字段逐项注释; 可选能力通过指针字段表达"未设置".

### 基础设施能力复用

- 编译期/运行期开关统一走 `base/env`(`env.Get().Dev()/Prod()/Debug()`), 不要在业务中散落硬编码判断.
- 命令行参数统一走 `base/flags`(注册、`Parse`、`AddParsedFunc` 回调), 需要 flag 覆盖配置时实现 `config.ConfigWithFlags`.
- 日志统一使用 `base/logger.CreateLogger` 创建 `glog.Logger`, 使用结构化字段而非字符串拼接.
- 超时控制优先使用 `utils/ctxutils` 封装的 `context.WithTimeout`(调试模式会自动放宽超时).
- 优雅退出使用 `utils/shutdown.go` 监听 `SIGINT` / `SIGTERM`.
- HTTP 调用使用 `utils/httputils`(请求选项 + 响应检查器, 含 `PostJson` / `GetJson`), 而非直接裸调 `http.Client`.

### 具体模块要点

- **base/protocol**: 协议 `PID` 由消息全名 FNV 哈希生成; 新增协议消息需注册到注册表, 保证编解码两侧一致.
- **base/codec**: c2s 包头含 `Pt` 表(请求/响应/推送)、`Seq` 序号、`PID`; s2s 负载为 4 字节 PID + proto 负载. 修改包头结构需同步更新 `head.go` 与相关测试.
- **infra/actor**: 是对 `gactor` 的工程化封装, 对外提供 RPC/Cast/Forward 与协议注册、模型持久化、位置注册表、路由等. 新增能力优先在此层做语法糖封装 (参考 `ContextSugarUtil` / `ActorSugarUtil`), 不改动底层 `gactor`.
- **infra/actor/metrics**: 基于 Prometheus 实现 `gactor.ServiceMonitor` / `gactor.ServiceMigrationMonitor`, 只负责指标采集; 指标暴露入口在 `infra/monitor/prometheus`.
- **infra/cluster**: `Center` 负责 etcd 节点注册/监听 (租约保活 + 退避重试), 涉及网络重试逻辑需保持幂等与可观测.
- **infra/mongobd**: `BD` 按哈希键将 `OpLoad`/`OpUpdate`/`OpBulk` 分发到多个 worker, 注意并发控制、默认超时与 panic 保护.
- **infra/noderouter**: 基于 rendezvous hashing 的节点选择; 节点集合变更通过 `Set`(全量/增量) 与 `Update`(有序增量) 维护.
- **infra/monitor**: `pprof` 注册标准路由; `probe` 提供 `/healthz`、`/readyz`, 新增就绪检查器遵循既有 `option` 模式; `prometheus` 提供服务层指标暴露入口 (`NewRegistry` 构建注册表、`RegisterHandler` 注册 `/metrics`).

### 测试

- 使用标准库 `testing`, 用例命名 `TestXxx`, 需要多场景时用 `t.Run` 子测试.
- 表驱动测试优先; 涉及随机/哈希的断言应关注"一致性/成员归属"而非固定值.
- 基准测试命名 `BenchmarkXxx`, 置于同包 `xxx_bench_test.go`.
- 新增或修改逻辑时同步补充/更新对应测试, 不删除既有测试来"通过".

## 依赖约束

- 核心依赖: `gactor`、`gcluster`、`glog`、`grendezvous`、`prometheus/client_golang`、etcd client、go-redis、mongo-driver、protobuf.
- 变更 `go.mod` 依赖版本需谨慎, 避免引入与现有分层冲突或重复能力的库; 优先复用已有依赖.
- 错误包装统一使用 `github.com/pkg/errors`, 不要额外引入其他错误库.

## 变更原则

- 只做与当前任务相关的改动, 不顺手重构无关代码, 不引入过度抽象.
- 修改公共 API 时同步更新调用方与 `README.md` 对应说明.
- 保持各分层职责边界的清晰: 基础能力放 `base`, 依赖外部中间件的编排放 `infra`, 纯函数工具放 `utils`.
