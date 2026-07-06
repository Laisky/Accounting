# 后端架构优化总体方案（变更手册）

- 状态：待评审骨架版。本手册只锁定方向性决策与验收口径，具体操作细节由开发组补全。
- 日期：2026-07-06
- 范围：`backend/`（Go API 服务器）的持久化、领域分层、API 契约、安全基线、可观测性、工具链与 CI；`cli/` 仅受 API 变更间接影响。
- 非目标：不改前端架构（见 `docs/proposals/2026-07-06-architecture-overhaul.md`，其后端侧契约工作在本手册落地）；不更换 Go/Gin；不做微服务拆分；不引入第三方 SaaS。
- 产品阶段：未上线，无生产数据，允许破坏性重构。这是以最低成本落地关系型 schema 与复式内核的唯一窗口期。

## 1. 背景

### 1.1 现状评估——已达标、需保留的设计

以下能力已符合 2026 最佳实践，重构中必须原样保留，不要重做：

| 项 | 证据 |
| --- | --- |
| 金额用 `int64` 分单位，汇率用十进制字符串 + `math/big.Rat` 半进位取整，全程无 float | `internal/ledger/model.go:259`、`internal/ledger/money.go:27-122` |
| SQL 全参数化，无字符串拼接注入面 | `internal/persistence/sql_store.go:114-205` |
| 会话为不透明随机令牌，库中只存 SHA-256 哈希；HttpOnly/SameSite=Lax/Secure 默认开 | `internal/auth/session.go:13-45` |
| 密码/TOTP/SSO state 全部常数时间比较；邮箱枚举防护与一次性验证码尝试上限 | `internal/auth/password.go:59`、`email_codes.go:190-221` |
| 严格 JSON 解码（拒绝未知字段）、查询参数白名单、分页上限 100 | `internal/httpserver/routes.go:439-514` |
| 秘密仅来自环境变量，无落盘默认秘密，审计元数据脱敏 | `internal/config/config.go:150-240`、`internal/audit/service.go:118-150` |
| 授权模型完整：全部已认证 CRUD 均在服务层复查 book 成员角色与资源属主，未发现越权缺口 | `internal/ledger/books.go:230-250`、`service.go:151-186` |
| 依赖方向无环，存在真实 service 层；`http.Server` 四类超时齐备 | `internal/httpserver/server.go:94-101` |

### 1.2 核心架构问题

| ID | 问题 | 证据 | 风险 |
| --- | --- | --- | --- |
| B1 | SQL 持久化是单张通用 JSON 表 `accounting_records`（namespace + JSON blob），无外键、无 CHECK、无法 SQL 聚合与过滤 | `internal/persistence/sql_store.go:342-389` | 记账数据无数据库级完整性保证；报表/对账无法下推数据库 |
| B2 | 无版本化迁移：启动时裸跑 `CREATE TABLE IF NOT EXISTS`，无迁移表 | `internal/persistence/sql_store.go:325-389` | schema 演进不可控、不可回滚 |
| B3 | 每个域三套 Store 实现（memory/file/SQL）互相复制 CRUD/排序/唯一性逻辑，驱动选择 switch 在 `httpserver` 复制 5 处 | `internal/httpserver/server.go:130-220`、`import_service.go:18-44` | 每加一个字段改 3 处；`httpserver` 成为持久化装配枢纽 |
| B4 | 跨 store 无事务：Wacai 导入 apply 逐行建 entry 后才单独 `MarkApplied`，中途失败重放会重复记账；并发 apply 可双双通过 `Status!=Applied` 门 | `internal/httpserver/import_routes.go:126-156, 226-236` | 数据重复写入，直接违背"正确性优先"产品目标 |
| B5 | 导入→记账编排（建账户/分类/条目/成员解析）全部写在 HTTP 层，`import_routes.go` 730 行 | `internal/httpserver/import_routes.go:215-600` | 领域规则无法在服务层测试；主要分层违规点 |
| B6 | 列表全量加载后内存分页，SQL 层无任何 `LIMIT/OFFSET` | `internal/ledger/pagination.go:17-35`、`entries.go:38-60` | 数据量增长后每次列表请求全表扫描 |
| B7 | 无 API 版本前缀（仅 `/api`）；错误响应 96 处 `{"error": "<泛化字符串>"}`，无错误码；真实错误仅 Debug 级日志 | `internal/httpserver/routes.go:101, 557-572` | 未来破坏性变更无缓冲；生产（Info 级）故障不可见 |
| B8 | 无 metrics（无 meter、无 /metrics）；OTel 仅 trace 且默认关 | `internal/telemetry/telemetry.go` | 无 RED/延迟/吞吐信号，无法做 SLO 告警 |
| B9 | 配置解析静默回退默认值（bool/int/duration 写错不报错），无启动期 `Validate()` | `internal/config/config.go:253-314` | 生产误配置无感知（如 TTL 拼错静默变 24h） |
| B10 | 汇率更新后台 goroutine 用 `context.Background()` 启动，shutdown 不回收 | `internal/ledger/rates.go:107-121`、`server.go:69` | goroutine 泄漏、退出不干净 |
| B11 | 非复式记账：转账是带可选目的账户的单条 entry，余额是服务层求和，无任何"借贷平衡"结构性不变量 | `internal/ledger/model.go:251-271`、`service.go:99-123` | 账户余额无法独立对账，报表完整性完全依赖求和代码路径 |
| B12 | 工具链薄弱：lint 仅 gofmt+vet，无 golangci-lint/gosec/govulncheck；Postgres 集成测试在 CI 因无 `DATABASE_URL` 被 skip，生产驱动从未在 CI 验证 | `Makefile:35-38`、`persistence/sql_store_test.go:42-47`、`.github/workflows/go.yml` | 静态缺陷类和 Postgres 特有行为（JSONB 转换、`$n` 重绑、部分唯一索引）零覆盖 |

### 1.3 安全审计要点（Medium 级，全部需在 P0 关闭）

| ID | 问题 | 证据 |
| --- | --- | --- |
| S1 | 密码重置 / TOTP 禁用后旧会话不吊销；Store 无按用户批量删除会话能力 | `auth/email_codes.go:110-139`、`totp.go:142-169`、`store.go:19-21` |
| S2 | 无账户锁定：失败计数只用于触发 Turnstile，防爆破仅靠进程内 IP+邮箱固定窗口限速（多副本不共享、换 IP 即新桶） | `auth/service.go:184-201`、`auth_rate_limiter.go:44-113` |
| S3 | 无 HSTS 响应头 | `httpserver/server.go:284-291` |
| S4 | JSON 路由无请求体大小上限（仅导入上传有 6 MiB 限制），存在内存耗尽 DoS 面 | `routes.go:439-452` |
| S5 | TOTP 秘密以明文（base32）随用户记录整体序列化落库/落盘，库泄露即 2FA 失效 | `auth/model.go:32-37` |
| S6 | 审计日志仅"约定式"追加：底层 `RecordStore` 可 Update/Delete，无哈希链；失败登录事件 ActorID 为空导致永远查不到，且无管理员全局审计视图 | `audit/sql_store.go:37-52`、`auth_routes.go:71-77` |
| S7 | 无成员移除/角色变更/取消共享 API——共享一旦发生不可撤销；`AddBookMember` 只能经导入侧路径触达 | `ledger/books.go:145-182`、`import_members.go:65-87` |
| S8 | SSO 一次性 token 走 URL query（会进访问日志），且自动开通用户默认开启 | `auth_sso_routes.go:72`、`config.go:176` |
| S9 | 密码哈希为 PBKDF2-SHA256（60 万次迭代）；OWASP 2026 首选 argon2id | `auth/password.go:16-24` |
| S10 | `FORCE_SMTP_TLS_VERIFY` 配置是死代码，实际发信路径为机会式 STARTTLS | `auth/email_sender.go:75-89` |

## 2. 总体设计决策（2026 基线）

| 领域 | 决策 | 理由 / 被否方案 |
| --- | --- | --- |
| 数据库 | 关系型 schema 取代单表 JSON：每个核心实体一张表，外键 + CHECK + 唯一约束落到数据库 | 记账产品的完整性必须由数据库背书；单表 JSON 阻塞 B1/B4/B6/B11 全部四项。否决"继续 JSON 表 + 应用层校验" |
| 驱动收敛 | 生产 PostgreSQL、单机自托管 SQLite、测试 memory；**废弃 file 驱动** | file 驱动是全量快照重写（`ledger/file_store.go:277-283`），SQLite 已覆盖其单机场景。三套实现减为"SQL 一套 + 测试用 memory 一套" |
| 迁移工具 | 版本化 SQL 迁移，嵌入二进制（`embed.FS`），启动可选自动迁移 + CLI 手动迁移。推荐 `pressly/goose` v3，备选 `golang-migrate` | SQL-first、可审查、可回滚；否决 Atlas 声明式（团队小、SQL 显式更稳）与 ORM 自动建表 |
| 事务模型 | 域仓储共享同一 `*sql.DB`，引入 UnitOfWork（`WithTx(ctx, fn)` 跨仓储）；导入 apply、注册等多写流程必须单事务 | 现状每域独立 `RecordStore` 使跨域事务不可能（`persistence/sql_store.go:250-270`） |
| 记账内核 | 分两步走向复式：先在关系 schema 中预留 `journal_entries` + `postings` 结构，转账改为同事务成对 posting；用户可见 Entry 模型不变 | 上线前落结构成本最低；余额可由 posting 独立对账。否决上线后再改（需数据迁移） |
| API | `/api/v1` 前缀；OpenAPI 3.1 契约由后端负责编写与验证（补上 `contract.yml` 占位）；错误响应统一为 RFC 9457 `application/problem+json`（含 `type/title/status/code/request_id`） | 与前端手册 Phase 1 的类型生成对接；泛化 error 字符串无法支撑客户端处理 |
| 密码哈希 | argon2id（`x/crypto/argon2`，OWASP 参数），登录成功时对旧 PBKDF2 哈希透明重哈希 | 无需强制重置密码即可迁移 |
| 静态敏感数据 | TOTP 秘密等服务端可逆秘密用 AES-256-GCM 信封加密，密钥来自 `ACCOUNTING_SECRET_KEY` 环境变量（预留 key id 支持轮换） | 库/备份泄露不再直接击穿 2FA |
| 可观测性 | OTel Metrics（已 stable）输出 RED 指标 + Go runtime 指标，OTLP 导出与现有 trace 共管道；新增 `/healthz`（存活）与 `/readyz`（含 DB ping） | 与前端手册 Phase 6 的遥测端点同一管道；否决第三方 APM |
| 工具链 | golangci-lint（含 errcheck/staticcheck/gosec）+ `govulncheck` 进 `make lint` 与 CI；CI 增加 Postgres service container 跑真实集成测试 | 仅 vet 的静态覆盖不足；生产驱动必须持续验证 |
| 配置 | 启动期 `Config.Validate()`：非法值直接 fail-fast 退出，禁止静默回退 | 记账服务宁可起不来，不可带错跑 |

## 3. 目标架构

```text
cmd/accounting-server        # 启动、信号、优雅退出（含全部后台 goroutine 的可取消 context）
internal/
├── httpserver               # 仅路由、鉴权中间件、DTO 编解码、problem+json 映射——不再装配存储、不再含领域编排
├── ledger | auth | audit    # 领域服务（不变量、策略），只依赖本域 Repository 接口
├── imports                  # 解析 + 预览 + 【新增】apply 编排（从 import_routes.go 下沉），依赖 ledger/auth 服务接口
├── storage                  # 【新增】统一装配：驱动选择一处、goose 迁移、UnitOfWork、各域 SQL Repository 实现
└── config / logger / telemetry / diagnostics
```

关系 schema 要点（细节由开发组在迁移文件中定稿）：

- 核心表：`users`、`sessions`、`books`、`book_members`、`categories`、`account_groups`、`accounts`、`entries`、`journal_entries`、`postings`、`exchange_rates`、`import_batches`、`import_rows`、`audit_events`。
- 必须落库的约束示例：`entries.amount_cents BIGINT CHECK (amount_cents > 0)`；`postings` 同一 `journal_entry_id` 下 `SUM(amount_cents) = 0`（延迟约束或事务内断言 + 定期对账任务）；`book_members(book_id, user_id)` 唯一；全部跨表引用用外键；时间列一律 `timestamptz`（UTC）。
- 列表查询下推数据库：`LIMIT/OFFSET` 起步，entry 流水预留 keyset 分页（`occurred_at, id` 复合索引）。
- 审计表只授予 INSERT/SELECT（应用连接角色层面），并加 `prev_hash` 哈希链列实现篡改可检。

## 4. 变更清单

按阶段推进；每阶段独立可评审、可回滚，P0 先行，P1 是其余阶段的地基。

### P0 —— 安全与工程地板（先行速赢，不依赖重构）

1. 新增 `Sessions.DeleteByUser`；密码重置、TOTP 禁用、（新增的）"登出全部设备"端点吊销该用户全部会话（S1）。
2. 账户锁定：连续失败 N 次后指数退避锁定 + 审计事件；限速器键策略修正（S2）。
3. HTTPS 响应加 HSTS；全部 JSON 路由套 `http.MaxBytesReader`（建议 1 MiB）（S3/S4）。
4. TOTP 秘密改为 AES-256-GCM 加密存储，提供一次性迁移路径（S5）。
5. 审计：失败认证事件补可查询主体（如小写邮箱哈希）；新增管理员全局审计读取；`audit_events` 加哈希链（哈希链可顺延至 P1 建表时做）（S6）。
6. 成员管理 API：移除成员、变更角色、取消账户共享，含审计（S7）。
7. SSO：回调 token 改 POST form（或至少对该路由禁用路径日志），`AUTO_PROVISION` 默认改 false（S8）。
8. argon2id + 登录透明重哈希（S9）；修复 SMTP TLS 强校验死代码（S10）。
9. `Config.Validate()` fail-fast（B9）；汇率后台任务接入可取消 context 并纳入优雅退出（B10）。
10. 工具链：引入 `.golangci.yml`（errcheck/staticcheck/gosec 等）、`govulncheck`，接入 `make backend-lint` 与 `go.yml`（B12）。

### P1 —— 持久化重构（本方案核心）

1. 新增 `internal/storage`：goose 嵌入式迁移、驱动选择单点化（删除 `httpserver` 内 5 处 switch）、UnitOfWork。
2. 编写首版关系 schema 迁移（第 3 节表清单与约束），Postgres 与 SQLite 双方言验证。
3. 按域实现 SQL Repository 替换 `RecordStore` JSON 读写；列表/汇总查询下推 SQL（B1/B6）。
4. 删除 file 驱动及各域 `SnapshotStore`；memory 实现仅保留给单测（B3）。
5. 数据迁移：提供 `cli` 子命令把现有 JSON 快照/`accounting_records` 数据导入新 schema（开发环境自用，无生产数据）。
6. CI：`go.yml` 增加 Postgres service container，集成测试不再 skip（B12）。

### P2 —— API 契约与错误模型

1. 路由迁移到 `/api/v1`（旧 `/api` 保留 302/别名一个过渡期，由前端同步切换）。
2. 编写 `docs/api/openapi.yaml`（OpenAPI 3.1）覆盖全部现有路由；`contract.yml` 从占位变为真实校验：spec lint + `httptest` 契约测试 + 前端生成类型新鲜度检查。
3. 统一错误模型：`application/problem+json`，稳定 `code` 枚举，携带 `request_id`；服务端对 5xx 记 Error 级日志（修正 B7 的 Debug 级黑洞）。
4. 移除未认证的遗留 demo 端点 `GET /api/ledger/summary`。

### P3 —— 导入域重构（依赖 P1 事务能力）

1. 把 apply 编排从 `import_routes.go`/`import_members.go` 下沉到 `internal/imports`（或独立 orchestrator），HTTP 层只剩 DTO 与鉴权（B5）。
2. apply 全流程单事务 + 数据库级幂等（batch 状态行 `SELECT ... FOR UPDATE` 或条件 UPDATE 抢占），杜绝重复记账与并发双写（B4）。
3. 行级失败语义明确化：全批回滚。

### P4 —— 可观测性与复式内核

1. OTel Metrics：HTTP RED、DB 池、导入批次、认证失败率；`/healthz`、`/readyz`（B8）。
2. 复式内核落地：entry 写入同事务生成平衡 posting（转账 = 两条 posting），提供余额对账查询与定期对账后台任务（B11）。
3. `docs/arch/arch.md` 持久化、安全、可观测性章节随实现更新。

## 5. 测试矩阵

| 层级 | 覆盖内容 | 工具 / 位置 | CI 门禁 |
| --- | --- | --- | --- |
| 静态 | lint、漏洞依赖、安全模式 | golangci-lint、govulncheck、gosec（golangci 内） | `go.yml`，0 error 必过 |
| 单元 | 领域不变量：金额/汇率取整、entry 校验、角色策略、posting 平衡 | `go test -race -cover`，memory 仓储 | 必过 |
| 迁移 | 每个迁移 up/down 可逆、从零到最新可重放、双方言（PG/SQLite） | goose + 集成测试 | 必过 |
| 仓储集成 | SQL Repository 全接口行为、外键/CHECK/唯一约束生效、分页下推 | SQLite（本地）+ Postgres service container（CI） | 必过，禁止 skip |
| 事务/并发 | 导入 apply 中断重放不重复；并发 apply 仅一次生效；会话批量吊销竞态 | 集成测试（注入故障点 + 并发 goroutine） | 必过 |
| 契约 | OpenAPI spec 与实现一致（状态码、schema、problem+json 结构）；路由 100% 被 spec 覆盖 | spec lint + `httptest` 契约测试 | `contract.yml` 必过 |
| 安全回归 | 密码重置后旧会话 401；锁定生效；超限 body 413；HSTS/CSP 头存在；TOTP 秘密落库为密文；SSO token 不入日志 | `httptest` + 集成测试 | 必过 |
| E2E | 注册→记账→导入→报表全流程不回退 | 现有 Playwright（`make e2e`） | `e2e.yml` 必过 |
| 性能烟测 | 10 万级 entries 下列表/汇总 p95 与内存上界 | `go test -bench` + Postgres 容器（阈值由开发组定） | 报告制，不阻塞 |

## 6. 验收标准

### 6.1 功能回归

- `make lint`、`make test`、`make e2e` 全绿；前端无需改动即可通过（P2 路由切换与前端同步例外）。
- 现有全部 API 行为（状态码、鉴权语义、分页语义）不回退，差异必须体现在 OpenAPI diff 中并经评审。

### 6.2 定量退出条件（逐条可机检）

1. 业务代码不再读写 `accounting_records` 表；每个核心实体有独立关系表且外键/CHECK 生效（对新库执行违约写入必须被数据库拒绝）。
2. schema 全部由版本化迁移管理；CI 在真实 Postgres 上执行迁移与集成测试，`t.Skip` 数为 0。
3. 全部路由带 `/api/v1` 前缀且 100% 出现在 `docs/api/openapi.yaml`；错误响应均为 problem+json 且含稳定 `code`。
4. 密码重置后，重置前签发的会话令牌请求返回 401。
5. 超过上限的 JSON 请求体返回 413；HTTPS 响应含 HSTS。
6. 直接 dump 数据库/快照文件，无法获得可用的 TOTP 秘密与任何明文口令材料。
7. 导入 apply 过程任意点 kill 进程后重试，最终 entry 数与批次行数一致（无重复、无缺失）；两个并发 apply 只有一个成功。
8. 每笔非转账 entry 对应一组和为零的 posting；转账在两账户产生成对 posting；对账查询能独立复算账户余额并与报表一致。
9. `httpserver` 包不再 import 任何 store 构造；`import_routes.go` 降到 300 行以下且不直接调用 `ledgerService.Create*`。
10. golangci-lint / govulncheck / gosec 全绿进 CI；metrics 端点/管道可观察到 HTTP RED 指标；`/readyz` 在 DB 断连时返回非 200。
11. 误配置任一 `ACCOUNTING_*` 布尔/时长/整数值时进程启动失败并指明变量名。

### 6.3 文档验收

- `docs/arch/arch.md` 的持久化、配置、安全、可观测性章节与实现同步更新；`file` 驱动从文档与配置表中移除。
- 新增 `docs/api/openapi.yaml` 为 API 唯一事实源；`docs/arch` 增加一页迁移与备份操作说明（含 SQLite→Postgres 路径）。

## 7. 风险与缓解

| 风险 | 缓解 |
| --- | --- |
| P1 重写仓储引入行为回归 | 先为现有 store 契约补齐行为测试，新旧实现跑同一套测试再切换；分域分 PR 落地 |
| 复式内核范围蔓延 | P4 只做"posting 结构 + 平衡不变量 + 对账查询"，报表切换到 posting 口径另立提案 |
| 前后端 `/api/v1` 切换窗口不同步 | P2 保留旧前缀别名一个发布周期，OpenAPI 同时描述别名弃用时间 |
| SQLite 与 Postgres 方言分叉 | 迁移与集成测试双方言强制跑；避免方言特有 SQL，必要处集中在 storage 层适配 |
