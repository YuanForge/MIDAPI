# 代理商体系与代理站点搭建开发执行文档

本文档用于后续 vibe coding / AI coding / 工程开发落地。目标是把代理商体系、代理站点搭建、价格同步、权限隔离、NATS 隔离、代理站运行模式整理成可拆任务执行的技术方案。

## 1. 总体结论

采用“同一代码仓库，两种运行模式，代理商控制台独立 Web”的方案。

```text
FanAPI 主站
  - app.mode 默认 master
  - 负责平台渠道、真实进价、平台售价、代理商管理、代理站搭建

代理商控制台
  - 单独 Web，建议 web/agent
  - 负责代理商登录、生成 Key、一步步搭建代理站

代理站点
  - app.mode = agent_site
  - 复制主站代码后以代理站模式运行
  - 使用自己的数据库、Redis DB、NATS namespace
  - 使用代理商 Key 对接主站 API
  - 只能设置利润倍率，不能修改主站渠道进价
```

不建议维护一个完全独立的新项目，也不建议直接把主站完整管理后台开放给代理商。

## 1.1 已确认产品规则

- 代理商目前只由平台管理员创建，不开放公开注册。
- 一个代理商可以创建多个代理站点。
- 代理商 Key 允许绑定多个代理站点。
- 代理商暂时不能自建第三方渠道；未来可以作为高级自营模式单独支持。
- 代理站不需要隐藏同步渠道，主站同步过来的可售渠道默认全部展示。
- 平台售价变化后，代理站需要实时查询 / 同步价格。
- 代理站创建失败后，已经创建的数据库和目录需要自动清理。
- 域名由代理商先解析到服务器；宿主机 Nginx 和域名由我们人工配置。
- 容器内部 Nginx 由系统自动配置。
- 代理站服务需要自动启动。
- 后期更新代理站时，由我们人工拉代码、build、执行 Docker Compose 更新。
- Redis DB 超过默认 16 个后，由运维扩容 Redis DB 数量或新建 Redis 实例。

## 2. 运行模式

### 2.1 配置规则

新增配置：

```yaml
app:
  mode: master
```

规则：

- 不配置 `app.mode` 时，默认等同于 `master`。
- `app.mode = master`：主站模式。
- `app.mode = agent_site`：代理站模式。
- 其他值启动失败。

代理站配置示例：

```yaml
app:
  mode: agent_site

platform_api:
  base_url: "https://主站API地址"
  key: "<agent-api-key>"
  price_sync_enabled: true

nats:
  namespace: "site_midcode_a1b2c3d4"
  task_stream: "TASKS_site_midcode_a1b2c3d4"
  task_subject: "site.midcode_a1b2c3d4.task.>"
  result_stream: "RESULTS_site_midcode_a1b2c3d4"
  result_subject: "site.midcode_a1b2c3d4.result.>"
```

### 2.2 主站模式能力

主站保留完整平台能力：

- 管理后台
- 用户端
- 平台渠道管理
- 第三方渠道添加
- 上游 Key / Key Pool 管理
- 真实渠道进价维护
- 平台售价维护
- 上游余额监控
- 上游成本同步
- 代理商管理
- 代理站点搭建
- 给代理站提供价格 / 渠道同步 API

### 2.3 代理站模式能力

代理站模式必须限制以下能力：

- 不开放主站完整渠道管理。
- 不允许新增第三方渠道。
- 不允许修改上游 API 地址。
- 不允许修改上游 API Key。
- 不允许编辑 request script / response script / error script。
- 不允许查看或修改主站真实渠道进价。
- 不启动主站 seed 渠道逻辑。
- 不启动主站上游成本同步。
- 不启动主站代理商管理功能。

代理站模式只保留：

- 终端用户注册、登录、充值、调用 API。
- 代理站自己的站点配置。
- 从主站同步来的模型 / 渠道展示。
- 代理商利润倍率设置。
- 代理站终端售价展示。
- 自己站点的数据统计。

## 3. Web 端划分

### 3.1 主站管理后台

给平台管理员使用。

新增菜单：

```text
代理商管理
  - 代理商列表
  - 代理站点列表
  - 搭建任务列表
```

主站管理员可做：

- 创建 / 禁用代理商。
- 管理员创建代理商账号，暂不开放代理商公开注册。
- 查看代理商 Key。
- 查看代理商已创建站点。
- 查看代理站搭建状态和失败原因。
- 查看代理站绑定的数据库、Redis DB、NATS namespace。
- 必要时重试搭建任务。

### 3.2 代理商控制台

建议单独 Web：

```text
web/agent
```

建议路由：

```text
/agent/login
/agent/dashboard
/agent/keys
/agent/sites
/agent/sites/new
/agent/sites/:id/build-progress
```

代理商控制台只做两类核心事情：

- 生成 Key
- 搭建代理站点

不要把代理站运行后的完整管理端塞进代理商控制台。代理商控制台是“建站控制台”，代理站管理端是站点建好后的日常运营后台。

### 3.3 代理站管理端

代理站建好后，代理商进入自己的代理站后台管理。

代理站后台的渠道页必须单独改成：

```text
同步渠道 / 利润设置
```

只允许：

- 查看主站同步来的模型 / 渠道。
- 查看代理商进价。
- 设置全局默认利润倍率。
- 设置单个渠道利润倍率。
- 查看代理站终端售价。

禁止：

- 新增第三方渠道。
- 隐藏主站同步渠道。
- 修改主站同步来的平台售价。
- 修改或查看主站真实渠道进价。
- 编辑上游 API 地址。
- 编辑上游 API Key。
- 编辑请求 / 响应脚本。
- 启动上游成本同步。

这些限制必须在后端接口层实现，不能只靠前端隐藏。

## 4. 代理商建站向导

代理商控制台里做一个分步向导。

### 4.1 步骤 1：生成或选择 Key

规则：

- 代理商必须先有启用状态的 Key，才能创建代理站。
- 没有 Key 时，“搭建代理站点”按钮置灰。
- Key 创建后完整值只展示一次，后续只展示掩码。

提示文案：

```text
请先生成 Key，再搭建代理站点。代理站点将使用该 Key 对接平台 API。
```

### 4.2 步骤 2：填写站点信息

字段：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| 网站名称 | 是 | 代理站显示名称 |
| Logo URL | 否 | 为空时使用默认 Logo |
| 站点编码 | 可自动生成 | 用于目录名、数据库名、NATS namespace |

### 4.3 步骤 3：填写 SMTP

字段：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| SMTP Host | 是 | 邮件服务器 |
| SMTP Port | 是 | 常见 465 / 587 |
| SMTP User | 是 | SMTP 账号 |
| SMTP Password | 是 | SMTP 密码或授权码 |
| SMTP From | 是 | 发件人 |

SMTP 密码需要加密保存，搭建日志不能输出明文。

### 4.4 步骤 4：设置默认利润倍率

字段：

| 字段 | 必填 | 默认值 |
| --- | --- | --- |
| 默认利润倍率 | 是 | 建议 1.7 |

规则：

- 最小值建议为 `1.0`。
- 代理站终端售价 = 代理商进价 × 利润倍率。
- 如果允许促销折扣，折扣后价格也不应低于代理商进价。

### 4.5 步骤 5：确认配置

提交前展示：

- 网站名称
- Logo URL
- 绑定 Key 掩码
- 默认利润倍率
- SMTP 信息，密码掩码
- 预计数据库名
- 预计 Redis DB
- 预计代码目录
- 预计 NATS namespace

### 4.6 步骤 6：开始搭建

后端创建异步搭建任务，代理商看到进度页。

状态：

| 状态 | 说明 |
| --- | --- |
| pending | 已提交，等待执行 |
| building | 搭建中 |
| success | 成功 |
| failed | 失败 |

失败时展示：

- 失败步骤
- 错误原因
- 是否可重试
- 联系管理员入口

## 5. 搭建执行流程

代理商提交后，后端执行：

1. 校验代理商身份。
2. 校验代理商是否有启用 Key。
3. 校验站点名称、Logo、SMTP、利润倍率。
4. 生成 `site_code`。
5. 生成目标代码目录。
6. 创建独立数据库。
7. 初始化代理站数据库表结构。
8. 分配独立 Redis DB。
9. 分配独立 NATS namespace。
10. 从 `/data/code/FanAPI` 复制代码到目标目录。
11. 写入代理站 `config.yaml`。
12. 写入代理站初始化数据。
13. 执行首次价格 / 渠道同步。
14. 自动配置容器内部 Nginx。
15. 自动启动代理站服务。
16. 更新搭建任务状态。

域名和宿主机 Nginx 不在自动搭建任务里完成：

1. 代理商先把域名解析到服务器。
2. 平台人工配置宿主机 Nginx 和域名证书。
3. 容器内部 Nginx 由代理站搭建流程自动配置。
4. 后续代理站更新由平台人工拉代码、build 并执行 Docker Compose 更新。
15. 更新搭建任务状态。

源目录：

```text
/data/code/FanAPI
```

目标目录示例：

```text
/data/code/midcode
```

更稳妥的目录规则：

```text
/data/code/{agent_id}_{8位随机英文数字}
```

## 6. 资源隔离

### 6.1 数据库

主站数据库：

```yaml
db:
  dbname: fanapi
```

代理站数据库命名：

```text
{site_code}_{8位随机英文数字}
```

示例：

```text
midcode_a1b2c3d4
```

规则：

- 只允许小写英文、数字、下划线。
- 长度不超过 PostgreSQL 标识符限制。
- 创建前检查是否存在。
- 搭建失败时记录已创建资源，支持清理或重试。
- 搭建失败时必须自动清理已经创建的数据库和代码目录。
- 清理失败时需要记录遗留资源，供管理员人工处理。

### 6.2 Redis

主站：

```yaml
redis:
  db: 0
```

代理站：

```text
第一个代理站：redis.db = 1
第二个代理站：redis.db = 2
以此类推
```

注意：

- Redis 默认通常只有 16 个 DB。
- 代理站数量可能超过 15 时，由运维扩容 Redis `databases` 数量或新建 Redis 实例。
- Redis DB 分配需要持久化记录，并加锁避免并发重复分配。

### 6.3 NATS

当前代码已经支持 NATS namespace 隔离。

主站示例：

```yaml
nats:
  namespace: master
  task_stream: TASKS_master
  task_subject: "master.task.>"
  result_stream: RESULTS_master
  result_subject: "master.result.>"
```

代理站示例：

```yaml
nats:
  namespace: "site_midcode_a1b2c3d4"
  task_stream: "TASKS_site_midcode_a1b2c3d4"
  task_subject: "site.midcode_a1b2c3d4.task.>"
  result_stream: "RESULTS_site_midcode_a1b2c3d4"
  result_subject: "site.midcode_a1b2c3d4.result.>"
```

规则：

- 可以共用同一台 NATS 服务。
- 主站和每个代理站必须使用不同 stream、subject、consumer。
- Worker 只订阅当前 namespace 的任务。
- 结果处理器只订阅当前 namespace 的结果。
- Worker 清理 consumer 时只能清理当前 namespace 的 consumer。

## 7. 价格体系

### 7.1 价格分层

| 价格层级 | 维护方 | 说明 |
| --- | --- | --- |
| 平台渠道进价 | 主站管理员 | 真实上游成本 |
| 平台售价 | 主站管理员 | 主站卖给代理商的价格 |
| 代理商进价 | 系统同步 | 固定等于平台售价 |
| 利润倍率 | 代理商 | 代理商唯一可调价格参数 |
| 代理站终端售价 | 系统计算 | 代理商进价 × 利润倍率 |

公式：

```text
代理商进价 = 平台售价
代理站终端售价 = 代理商进价 × 利润倍率
代理商利润 = 代理站终端售价 - 代理商进价
```

示例：

```text
平台渠道进价：0.50 CNY / 秒
平台售价：0.80 CNY / 秒
代理商进价：0.80 CNY / 秒
利润倍率：1.70
代理站终端售价：1.36 CNY / 秒
代理商利润：0.56 CNY / 秒
```

### 7.2 同步规则

- 代理站通过代理商 Key 调用主站 API。
- 主站只返回代理商可见的模型、渠道和平台售价。
- 主站不返回真实渠道进价。
- 代理站保存代理商进价快照和利润倍率。
- 主站平台售价变化后，代理站需要实时查询 / 同步最新价格，并重新计算终端售价。

## 8. 建议数据表

### 8.1 agents

| 字段 | 说明 |
| --- | --- |
| id | 代理商 ID |
| username / email / phone | 登录凭证 |
| password_hash | 密码哈希 |
| name | 代理商名称 |
| status | 启用 / 禁用 |
| created_at | 创建时间 |
| updated_at | 更新时间 |
| last_login_at | 最近登录时间 |

### 8.2 agent_sites

| 字段 | 说明 |
| --- | --- |
| id | 站点 ID |
| agent_id | 代理商 ID |
| site_name | 网站名称 |
| site_code | 站点编码 |
| logo_url | Logo URL |
| code_path | 代码目录 |
| domain | 代理站域名 |
| domain_status | pending_dns / dns_ready / nginx_configured |
| db_name | 数据库名 |
| redis_db | Redis DB |
| nats_namespace | NATS namespace |
| platform_api_key_id | 绑定 Key ID |
| default_profit_rate | 默认利润倍率 |
| smtp_host | SMTP Host |
| smtp_port | SMTP Port |
| smtp_user | SMTP User |
| smtp_password_encrypted | 加密 SMTP 密码 |
| smtp_from | 发件人 |
| compose_project | Docker Compose 项目名 |
| service_status | stopped / starting / running / failed |
| status | pending / building / success / failed |
| error_message | 失败原因 |
| created_at | 创建时间 |
| updated_at | 更新时间 |

### 8.3 agent_site_build_jobs

| 字段 | 说明 |
| --- | --- |
| id | 搭建任务 ID |
| site_id | 站点 ID |
| agent_id | 代理商 ID |
| current_step | 当前步骤 |
| status | pending / running / success / failed |
| error_message | 失败原因 |
| resources_json | 已创建资源记录，用于失败自动清理和人工兜底 |
| cleanup_status | none / cleaning / success / failed |
| created_at | 创建时间 |
| updated_at | 更新时间 |

### 8.4 agent_site_channel_prices

| 字段 | 说明 |
| --- | --- |
| id | 记录 ID |
| site_id | 代理站 ID |
| platform_channel_id | 主站渠道 ID |
| billing_type | 计费类型 |
| agent_cost_snapshot | 代理商进价快照，等于平台售价 |
| profit_rate | 利润倍率 |
| final_price_snapshot | 终端售价快照 |
| synced_at | 最近同步时间 |
| updated_at | 更新时间 |

## 9. 建议接口

### 9.1 主站管理后台接口

```text
GET    /admin/agents
POST   /admin/agents
PATCH  /admin/agents/:id/status
GET    /admin/agent-sites
GET    /admin/agent-sites/:id
GET    /admin/agent-site-build-jobs
POST   /admin/agent-sites/:id/retry-build
POST   /admin/agent-sites/:id/cleanup
```

### 9.2 代理商控制台接口

```text
POST   /agent/auth/login
GET    /agent/me
GET    /agent/keys
POST   /agent/keys
PATCH  /agent/keys/:id/status
GET    /agent/sites
POST   /agent/sites
GET    /agent/sites/:id
GET    /agent/sites/:id/build-progress
```

说明：

- 代理商账号由管理员创建，因此不提供公开 `/agent/register`。
- 一个代理商可以创建多个代理站点。
- 一个代理商 Key 可以绑定多个代理站点。

### 9.3 主站给代理站的同步接口

代理站使用代理商 Key 调用。

```text
GET /agent-platform/channels
GET /agent-platform/prices
GET /agent-platform/settings
```

返回内容必须过滤：

- 只返回平台售价，不返回真实渠道进价。
- 不返回上游 API Key。
- 不返回 request / response / error script，除非代理站调用能力必须用到且已经做脱敏和安全评估。

### 9.4 代理站模式接口限制

当 `app.mode = agent_site` 时：

- 禁用 `/admin/channels` 的新增、编辑、删除。
- 禁用 `/admin/channels/:id/sync-upstream-cost`。
- 禁用上游平台管理。
- 禁用 Key Pool 管理。
- 替换为代理站专用价格接口：

```text
GET   /agent-site/channels
PATCH /agent-site/channels/:id/profit-rate
POST  /agent-site/channels/sync
```

代理站不需要隐藏同步渠道，因此不提供渠道隐藏接口。

## 10. 初始化策略

当前主站启动时会执行较多初始化行为，例如：

- 自动同步表结构。
- seed 默认管理员。
- seed 默认渠道。
- ensure indexes。
- 启动上游余额监控。
- 启动上游成本监控。
- 启动 OCPC 调度。

代理站模式不能原样执行所有主站初始化。

建议拆分：

```text
master init
  - 主站表结构
  - 默认管理员
  - 默认渠道
  - 平台监控任务

agent_site init
  - 必要表结构
  - 站点配置
  - 绑定平台 API Key
  - 默认利润倍率
  - 首次同步主站渠道和售价
```

## 11. 安全要求

- 代理商只能访问自己的 Key 和站点。
- 代理站只能使用绑定的代理商 Key 调主站 API。
- 平台真实渠道进价不能下发到代理站。
- SMTP 密码、平台 API Key 不能明文二次展示。
- 搭建日志不能输出密码、Key、数据库密码。
- 站点目录名必须防路径穿越。
- 复制代码时排除 `.git`、缓存、日志、临时文件。
- 代理站模式下后端必须拦截主站敏感接口。

建议复制排除：

```text
.git/
.codex-run/
logs/
tmp/
node_modules/
web/*/node_modules/
uploads/cache/
```

## 12. 一期实现拆分

建议按顺序开发：

1. 增加 `app.mode` 和模式判断。
2. 拆分主站初始化和代理站初始化。
3. 增加代理商数据表和管理后台接口。
4. 增加代理商登录和 Key 管理。
5. 新建 `web/agent` 代理商控制台。
6. 实现代理商建站向导。
7. 实现建站异步任务。
8. 实现数据库、Redis DB、NATS namespace 分配。
9. 实现代理站配置生成。
10. 实现容器内部 Nginx 自动配置和代理站服务自动启动。
11. 实现主站给代理站的渠道 / 价格实时同步 API。
12. 实现创建失败自动清理数据库和目录。
13. 实现代理站模式下的渠道页面限制。
14. 实现代理站“同步渠道 / 利润设置”页面。
15. 实现搭建任务重试和失败原因展示。

## 13. 验收标准

### 13.1 主站不受影响

- 不配置 `app.mode` 时主站正常启动。
- 主站原有用户、管理后台、渠道、任务功能正常。
- 主站 NATS 使用 `master` namespace。

### 13.2 代理商控制台

- 代理商可以登录。
- 代理商可以生成 Key。
- 没有 Key 时不能搭建代理站。
- 一个代理商可以创建多个代理站。
- 一个代理商 Key 可以绑定多个代理站。
- 搭建向导可以完整提交。
- 可以看到搭建进度和失败原因。

### 13.3 代理站

- 代理站使用独立数据库。
- 代理站使用独立 Redis DB。
- 代理站使用独立 NATS namespace。
- 代理站可以同步主站模型和平台售价。
- 代理站不能看到主站真实渠道进价。
- 代理站不能新增第三方渠道。
- 代理站不能隐藏同步渠道。
- 代理站只能设置利润倍率。
- 代理站终端售价计算正确。
- 主站平台售价变化后，代理站可以实时查询 / 同步最新价格。
- 代理站创建失败后，已创建数据库和目录会自动清理。

### 13.4 隔离验证

- 主站任务不会被代理站 Worker 消费。
- 代理站任务不会被主站 Worker 消费。
- 代理站结果不会写入主站数据库。
- 一个代理站出错不影响其他站点。

## 14. 暂不做和未来扩展

### 14.1 一期暂不做

- 代理商公开注册。
- 代理站隐藏同步渠道。
- 代理商自建第三方渠道。
- 代理商高级自营模式。
- 域名和宿主机 Nginx 全自动配置。
- 代理站后续版本自动升级。

### 14.2 未来可扩展

- 高级自营模式：允许特定代理商自建第三方渠道。
- 域名、证书、宿主机 Nginx 自动化。
- 多 Redis 实例自动分配。
- 代理站批量升级和自动发布。
