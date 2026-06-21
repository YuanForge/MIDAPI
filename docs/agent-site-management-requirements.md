# 代理商管理与代理站点搭建需求文档

## 1. 背景

当前 FanAPI 需要新增代理商体系。平台管理员可以在管理后台维护代理商用户；代理商注册、登录后，只保留两个核心功能：

- 生成 Key
- 搭建代理站点

代理站点搭建的核心目标是：代理商在页面填写站点信息后，系统自动从服务器现有线上项目目录复制一份新的站点代码，并自动生成该站点独立的数据库、Redis DB 编号和配置文件。

代理商必须先生成 Key，才能创建代理站点。代理站点创建成功后，默认使用该 Key 对接平台 API，从平台实时同步模型、渠道、售价和价格变动。代理商不能修改平台渠道进价，只能在自己的代理站点内设置利润倍率。

## 2. 用户角色

### 2.1 平台管理员

平台管理员在管理后台新增“代理商管理”模块，用于管理所有代理商账号和代理站点。

建议包含功能：

- 代理商列表
- 新增代理商
- 编辑代理商信息
- 启用 / 禁用代理商
- 重置代理商密码
- 查看代理商已创建的代理站点
- 查看代理商 Key 使用情况
- 查看代理站点搭建状态和失败原因

### 2.2 代理商用户

代理商用户注册、登录后，只能看到代理商后台。

代理商后台只开放两个功能：

- 生成 Key
- 搭建代理站点

代理商不应该看到平台管理后台、其他代理商数据、平台全局配置等敏感信息。

## 3. 功能范围

### 3.1 管理后台新增代理商管理

菜单建议：

```text
管理后台
  - 代理商管理
    - 代理商列表
    - 代理站点列表
```

代理商列表建议字段：

| 字段 | 说明 |
| --- | --- |
| 代理商 ID | 系统生成的唯一 ID |
| 账号 / 邮箱 / 手机号 | 登录凭证，按现有用户体系选择 |
| 昵称 / 名称 | 代理商显示名称 |
| 状态 | 启用、禁用 |
| 已建站点数 | 当前代理商创建的代理站点数量 |
| 创建时间 | 代理商创建时间 |
| 最近登录时间 | 便于排查账号使用情况 |

代理站点列表建议字段：

| 字段 | 说明 |
| --- | --- |
| 站点 ID | 系统生成的唯一 ID |
| 代理商 ID | 归属代理商 |
| 网站名称 | 代理商填写的网站名字 |
| 站点编码 | 用于目录名、配置名或内部识别 |
| Logo URL | 可为空 |
| 代码目录 | 例如 `/data/code/midcode` |
| 数据库名 | 自动生成 |
| Redis DB | 自动分配 |
| NATS 地址 | 当前配置使用的 NATS 地址 |
| SMTP 状态 | 是否已配置 |
| 搭建状态 | 待搭建、搭建中、成功、失败 |
| 失败原因 | 失败时记录详细错误 |
| 创建时间 | 站点创建时间 |

### 3.2 代理商注册登录

代理商需要有独立登录入口，建议与普通用户、管理员入口隔离。

示例：

```text
/agent/register
/agent/login
/agent/dashboard
```

登录后权限限制：

- 只能访问代理商后台接口
- 只能操作自己的 Key
- 只能操作自己的代理站点
- 不能访问管理后台接口
- 不能查看或修改其他代理商的站点信息

### 3.3 生成 Key

代理商可以生成自己的 Key。

Key 是代理站点对接平台 API 的凭证，也是创建代理站点的前置条件。

建议规则：

- Key 归属于当前代理商
- 每个代理商至少需要有一个启用状态的 Key，才允许提交“搭建代理站点”
- 创建代理站点时必须选择一个已启用的 Key，或默认使用最近创建的启用 Key
- 代理站点的上游 API 请求统一使用该 Key 调用平台 API
- Key 可启用 / 禁用
- Key 可设置名称备注
- Key 需要记录创建时间、最近使用时间
- Key 不应明文二次展示完整值，只在创建时展示一次，后续只展示掩码

如果当前系统已有 Key 体系，建议复用现有逻辑，只增加 `agent_id` 或等价归属字段。

代理商没有 Key 时，“搭建代理站点”按钮建议置灰，并提示：

```text
请先生成 Key，再搭建代理站点。代理站点将使用该 Key 对接平台 API。
```

### 3.4 搭建代理站点

代理商点击“搭建代理站点”后，进入搭建流程页面。

进入搭建流程前，系统必须先检查当前代理商是否存在可用 Key。

页面表单字段：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| 网站名字 | 是 | 用于前台展示，也用于生成数据库名的一部分 |
| Logo URL | 否 | 可为空；为空时使用系统默认 Logo |
| 平台 API Key | 是 | 代理站点用于对接平台 API 的 Key；默认选中当前代理商可用 Key |
| SMTP Host | 是 | 代理站点自己的 SMTP 服务器 |
| SMTP Port | 是 | 常见为 `465` 或 `587` |
| SMTP User | 是 | SMTP 登录账号 |
| SMTP Password | 是 | SMTP 密码或授权码 |
| SMTP From | 是 | 发件人展示名称和邮箱 |

SMTP 示例配置：

```yaml
smtp:
  host: smtp.example.com
  port: 465
  user: no-reply@example.com
  password: "<smtp-password>"
  from: "Site Name <no-reply@example.com>"
```

### 3.5 代理商价格与利润倍率

这个模式可以成立，建议按“平台售价 = 代理商进价”的规则设计。

价格分层：

| 价格层级 | 维护方 | 说明 |
| --- | --- | --- |
| 平台渠道进价 | 平台管理员 | 平台真实上游成本，只在平台管理后台维护 |
| 平台售价 | 平台管理员 | 平台卖给代理商的价格 |
| 代理商进价 | 系统自动同步 | 固定等于平台售价，代理商不能修改 |
| 代理商利润倍率 | 代理商 | 代理商唯一可以调整的价格参数 |
| 代理站终端售价 | 系统计算 | 代理商进价 × 代理商利润倍率 |

计算公式：

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
代理商利润倍率：1.70
代理站终端售价：0.80 × 1.70 = 1.36 CNY / 秒
代理商利润：1.36 - 0.80 = 0.56 CNY / 秒
```

权限限制：

- 代理商不能查看或修改平台渠道进价。
- 代理商不能修改自己的进价，因为代理商进价必须跟随平台售价。
- 代理商只能修改利润倍率。
- 利润倍率建议设置最小值，例如 `1.0`，避免代理商低于成本销售。
- 如果允许代理商做促销，可以单独设计折扣功能，但折扣后的终端售价不应低于代理商进价。

价格同步规则：

- 代理站点通过代理商 Key 调用平台 API，同步平台可售模型和平台售价。
- 平台售价变动后，代理站点应自动更新代理商进价。
- 代理站点终端售价需要按最新代理商进价和代理商利润倍率重新计算。
- 代理站本地只保存利润倍率和必要的价格快照，不保存平台渠道真实进价。

代理站点的渠道编辑页面建议调整：

- “进价”字段展示为只读，值来自平台售价。
- 隐藏平台渠道成本字段。
- 保留“利润倍率”输入框。
- 页面文案建议从“售价 = 成本 × 倍率”调整为“终端售价 = 代理商进价 × 利润倍率”。

## 4. 搭建流程

### 4.1 流程概览

代理商提交站点信息后，系统按以下流程执行：

1. 校验代理商身份和权限
2. 校验当前代理商是否存在启用状态的 Key
3. 校验网站名字、Logo URL、SMTP 信息
4. 绑定代理站点使用的平台 API Key
5. 生成站点编码
6. 生成目标代码目录
7. 从 `/data/code/FanAPI` 复制一份代码到目标目录
8. 生成独立数据库名
9. 在 PostgreSQL 中创建该数据库
10. 分配独立 Redis DB 编号
11. 修改目标目录下的 `config.yaml`
12. 初始化代理站点价格同步配置
13. 记录站点搭建状态
14. 展示搭建结果页面

### 4.2 代码目录复制规则

源目录固定为：

```text
/data/code/FanAPI
```

目标目录示例：

```text
/data/code/midcode
```

建议目录名生成规则：

```text
/data/code/{site_code}
```

`site_code` 建议由网站名字转换得到：

- 只保留小写英文、数字、短横线或下划线
- 中文或特殊字符可转拼音，或直接使用系统生成的短编码
- 不允许出现路径穿越字符，例如 `../`
- 如果目录已存在，需要追加随机后缀或提示用户更换名称

示例：

```text
网站名字：MID CODE
站点编码：midcode
目标目录：/data/code/midcode
```

如果要更稳妥，建议直接使用：

```text
/data/code/{agent_id}_{8位随机英文数字}
```

这样可以避免中文、重名、特殊字符导致目录冲突。

### 4.3 数据库名生成规则

线上主库当前为：

```yaml
db:
  dbname: fanapi
```

代理站点需要自动创建新的 `dbname`，不能继续使用 `fanapi`。

建议生成规则：

```text
{站点编码}_{8位随机英文数字}
```

示例：

```text
网站名字：MID CODE
站点编码：midcode
随机后缀：a1b2c3d4
数据库名：midcode_a1b2c3d4
```

数据库名限制：

- 只允许小写英文、数字、下划线
- 长度建议不超过 63 个字符，兼容 PostgreSQL 标识符长度
- 必须检查是否已存在
- 创建失败时必须回滚站点搭建状态，并记录失败原因

建议执行逻辑：

1. 生成数据库名
2. 连接 PostgreSQL 管理库
3. 检查数据库是否存在
4. 不存在则创建数据库
5. 将目标站点 `config.yaml` 中的 `db.dbname` 改为新数据库名

是否需要初始化表结构，需要根据现有项目启动逻辑确认：

- 如果服务启动时自动迁移表结构，则创建数据库即可
- 如果没有自动迁移，则搭建流程需要额外执行初始化 SQL 或迁移命令

### 4.4 Redis DB 分配规则

当前线上 Redis 使用：

```yaml
redis:
  db: 0
```

代理站点需要使用独立 Redis DB。

分配规则：

```text
第一个代理站点：redis.db = 1
第二个代理站点：redis.db = 2
第三个代理站点：redis.db = 3
以此类推
```

注意事项：

- Redis 默认通常只有 `0-15` 共 16 个 DB。
- 如果代理站点数量可能超过 15 个，需要提前调整 Redis `databases` 配置，或改用 key prefix 隔离。
- Redis DB 分配必须持久化记录，不能只靠扫描配置文件。
- 分配 Redis DB 时需要加锁，避免两个代理商同时搭建时拿到同一个 DB。

建议新增一张资源分配表，记录每个站点占用的 Redis DB：

| 字段 | 说明 |
| --- | --- |
| site_id | 站点 ID |
| redis_db | Redis DB 编号 |
| allocated_at | 分配时间 |
| status | 占用、释放 |

### 4.5 NATS 配置建议

当前示例配置：

```yaml
nats:
  url: "nats://127.0.0.1:4222"
```

服务器上可以只部署一个 NATS 服务，但主站和每个代理站点不能直接共用同一套全局 stream、subject 和 consumer。

当前代码中的 NATS 风险点：

- 任务 stream 固定为 `TASKS`
- 结果 stream 固定为 `RESULTS`
- 任务 subject 固定为 `task.>`
- 结果 subject 固定为 `result.>`
- Worker 默认订阅 `task.>`
- 结果处理器默认订阅 `result.>`
- Worker 启动时会清理 `TASKS` stream 下的 consumer

如果主站和多个代理站都连接同一个 NATS，并继续使用这些全局名称，会出现以下问题：

- 代理站 Worker 可能消费到主站任务
- 主站 Worker 可能消费到代理站任务
- 代理站结果可能被主站结果处理器写入主站数据库
- Worker 启动时清理 consumer，可能影响其他站点正在运行的 Worker
- 多个站点使用相同 durable consumer 名称，可能导致订阅冲突

因此，NATS 可以公用服务实例，但必须做逻辑隔离。

可以公用的前提：

- 每个站点有独立 NATS 命名空间
- 每个站点的任务 subject 有站点维度隔离，例如 `site.{site_id}.task.video`
- 每个站点的结果 subject 有站点维度隔离，例如 `site.{site_id}.result.{task_id}`
- 每个站点使用独立 stream，或至少 stream subject 不重叠
- 每个站点使用独立 durable consumer 名称
- Worker 消费时不会把 A 站点任务处理到 B 站点
- 任务 payload 中包含站点 ID 或租户 ID
- 日志、回调、结果写入都能回到正确数据库

不建议直接公用的情况：

- 当前 subject 是全局的，例如 `task.video.*`
- Worker 不区分站点或租户
- Worker 只连接单一数据库，无法按任务切换数据库
- 任务结果没有站点归属字段
- Worker 会清理全局 stream 下的 consumer

推荐方案：

- 继续使用同一个 NATS 服务地址。
- 主站和每个代理站点使用不同 namespace。
- 每个 namespace 使用独立的 `TASKS` / `RESULTS` stream 名称。
- 每个 namespace 使用独立 subject 前缀。
- 每个 namespace 使用独立 consumer 名称。
- 每个代理站点独立启动自己的 server 和 worker，连接自己的数据库与 Redis DB。

示例配置：

```yaml
nats:
  url: "nats://127.0.0.1:4222"
  namespace: "site_midcode_a1b2c3d4"
  task_stream: "TASKS_site_midcode_a1b2c3d4"
  task_subject: "site.midcode_a1b2c3d4.task.>"
  result_stream: "RESULTS_site_midcode_a1b2c3d4"
  result_subject: "site.midcode_a1b2c3d4.result.>"
```

发布任务 subject 示例：

```text
site.midcode_a1b2c3d4.task.video.123
site.midcode_a1b2c3d4.task.image.456
```

发布结果 subject 示例：

```text
site.midcode_a1b2c3d4.result.10001
```

Worker 订阅示例：

```yaml
worker:
  subjects:
    - "site.midcode_a1b2c3d4.task.>"
```

主站也建议改成显式 namespace，例如：

```yaml
nats:
  namespace: "master"
  task_stream: "TASKS_master"
  task_subject: "master.task.>"
  result_stream: "RESULTS_master"
  result_subject: "master.result.>"
```

代码改造要求：

- `TASKS`、`RESULTS`、`task.>`、`result.>` 不能继续写死，需要从配置读取。
- 发布任务时根据当前站点 namespace 拼接 subject。
- 发布结果时根据当前站点 namespace 拼接 subject。
- 结果处理器只订阅当前站点的 result subject。
- Worker 只订阅当前站点的 task subject。
- consumer 名称必须带 namespace，例如 `site_midcode_a1b2c3d4_workers_all`。
- `PurgeConsumers` 只能清理当前站点 stream 下的 consumer，不能影响其他站点。

待技术确认：

- 当前 `fanapi-script` Worker 是否只读取本地 `config.yaml` 中的一套数据库配置。
- 当前 NATS subject 是否有租户 / 站点隔离字段。
- 一个 Worker 是否能同时处理多个代理站点的任务。

一期建议不做“一个 Worker 处理多个站点”。更稳妥的方式是每个站点启动自己的 Worker，Worker 只连接当前站点数据库、Redis DB 和 NATS namespace。

## 5. `config.yaml` 自动修改规则

复制代码后，需要修改目标目录下的：

```text
{目标目录}/config.yaml
```

需要自动改动的字段：

```yaml
db:
  dbname: "{自动生成的新数据库名}"

redis:
  db: {自动分配的Redis DB编号}

nats:
  url: "nats://127.0.0.1:4222"

smtp:
  host: "{代理商填写的SMTP Host}"
  port: {代理商填写的SMTP Port}
  user: "{代理商填写的SMTP User}"
  password: "{代理商填写的SMTP Password}"
  from: "{代理商填写的SMTP From}"
```

如果后续配置文件支持平台 API 上游配置，建议同时写入：

```yaml
platform_api:
  base_url: "https://平台主站API地址"
  key: "{代理商选择的平台API Key}"
  price_sync_enabled: true
```

如果不希望 Key 明文写入 `config.yaml`，可以改为写入环境变量或独立密钥文件，并限制文件权限。

不建议自动改动的字段：

- `db.host`
- `db.port`
- `db.user`
- `db.password`
- `redis.addr`
- `redis.password`

除非后续确定每个代理站点也要使用独立数据库账号或独立 Redis 实例。

## 6. 搭建状态设计

因为搭建过程涉及复制文件、创建数据库、改配置文件，建议作为异步任务处理。

状态建议：

| 状态 | 说明 |
| --- | --- |
| pending | 已提交，等待执行 |
| building | 搭建中 |
| success | 搭建成功 |
| failed | 搭建失败 |

失败时需要记录：

- 失败步骤
- 错误信息
- 是否可以重试
- 已创建的资源有哪些

搭建任务建议支持重试，但要保证幂等：

- 目录已存在时不能直接覆盖
- 数据库已存在时需要确认是否属于当前站点
- Redis DB 已分配时不能重复分配
- `config.yaml` 修改失败时需要保留原始备份

## 7. 安全要求

- 代理商只能操作自己的站点和 Key。
- SMTP 密码需要加密保存，不能明文展示。
- 平台 API Key 只能在创建时完整展示一次，代理站点配置中应避免明文泄露。
- 平台渠道真实进价不能同步到代理站点，也不能出现在代理商接口返回值中。
- 代理商只能设置利润倍率，不能修改代理商进价和平台售价。
- 站点目录名必须严格校验，防止路径穿越。
- 复制目录时需要排除不应该复制的文件，例如运行日志、临时文件、上传缓存、`.git`、本地构建缓存等。
- 不建议把生产 SMTP 密码、数据库密码写入公开文档或前端页面。
- 搭建任务日志不能输出完整密码、Key、数据库密码。
- 后台接口必须校验管理员权限，代理商接口必须校验代理商权限。

建议复制目录时排除：

```text
.git/
.codex-run/
logs/
tmp/
node_modules/
web/*/node_modules/
uploads/cache/
```

具体排除列表需要根据线上目录实际内容确认。

## 8. 页面流程建议

### 8.1 代理商首页

代理商登录后进入首页，只展示两个入口：

```text
[生成 Key] [搭建代理站点]
```

如果代理商还没有启用状态的 Key：

- “生成 Key”按钮可正常点击
- “搭建代理站点”按钮置灰
- 页面提示“请先生成 Key，再搭建代理站点”

### 8.2 搭建代理站点页面

页面步骤：

1. 检查代理商是否已有可用 Key
2. 选择代理站点使用的平台 API Key
3. 填写站点信息
4. 填写 SMTP 信息
5. 确认配置
6. 提交搭建
7. 展示搭建进度
8. 展示搭建结果

搭建结果成功时展示：

- 网站名字
- 站点目录
- 数据库名
- Redis DB
- 已绑定的平台 API Key 掩码
- 访问地址
- 创建时间

搭建失败时展示：

- 失败步骤
- 失败原因
- 重试按钮
- 联系管理员入口

## 9. 数据表建议

### 9.1 代理商表

```text
agents
```

建议字段：

| 字段 | 说明 |
| --- | --- |
| id | 代理商 ID |
| username / email / phone | 登录凭证 |
| password_hash | 密码哈希 |
| name | 代理商名称 |
| status | 状态 |
| created_at | 创建时间 |
| updated_at | 更新时间 |
| last_login_at | 最近登录时间 |

### 9.2 代理站点表

```text
agent_sites
```

建议字段：

| 字段 | 说明 |
| --- | --- |
| id | 站点 ID |
| agent_id | 代理商 ID |
| site_name | 网站名字 |
| site_code | 站点编码 |
| logo_url | Logo URL |
| code_path | 代码目录 |
| db_name | 数据库名 |
| redis_db | Redis DB 编号 |
| nats_url | NATS 地址 |
| smtp_host | SMTP Host |
| smtp_port | SMTP Port |
| smtp_user | SMTP User |
| smtp_password_encrypted | 加密后的 SMTP 密码 |
| smtp_from | 发件人 |
| platform_api_key_id | 绑定的平台 API Key ID |
| platform_api_key_masked | 平台 API Key 掩码，仅用于展示 |
| price_sync_enabled | 是否启用价格同步 |
| default_profit_rate | 默认利润倍率 |
| status | 搭建状态 |
| error_message | 失败原因 |
| created_at | 创建时间 |
| updated_at | 更新时间 |

### 9.3 Key 表归属字段

如果现有 Key 表已经存在，建议增加代理商归属字段：

| 字段 | 说明 |
| --- | --- |
| agent_id | 代理商 ID |
| site_id | 可选，Key 是否绑定某个代理站点 |

### 9.4 代理站点价格配置表

```text
agent_site_channel_prices
```

建议字段：

| 字段 | 说明 |
| --- | --- |
| id | 记录 ID |
| site_id | 代理站点 ID |
| platform_channel_id | 平台渠道 ID |
| billing_type | 计费类型 |
| agent_cost_snapshot | 代理商进价快照，等于平台售价 |
| profit_rate | 代理商设置的利润倍率 |
| final_price_snapshot | 代理站终端售价快照 |
| synced_at | 最近同步时间 |
| updated_at | 更新时间 |

说明：

- `agent_cost_snapshot` 只保存平台售价快照，不保存平台渠道真实进价。
- `profit_rate` 是代理商可编辑字段。
- `final_price_snapshot` 可由同步任务计算，也可以请求时实时计算。

## 10. 待确认问题

以下问题需要在开发前确认：

1. 代理商注册是否开放给所有人，还是只能管理员手动创建？
2. 一个代理商是否允许创建多个代理站点？
3. 代理站点是否需要独立域名绑定？
4. 复制代码后是否需要自动启动新站点服务？
5. 新站点端口、进程管理、Nginx 配置是否需要系统自动生成？
6. 数据库创建后是否有自动迁移机制？
7. Redis DB 数量是否会超过默认 16 个？
8. NATS 是否需要每个站点独立，还是通过 subject 隔离共用？
9. Worker 是否需要每个站点独立启动？
10. SMTP 是否必须由代理商填写，还是可以先使用平台默认 SMTP？
11. Logo URL 为空时使用哪个默认 Logo？
12. 代理站点创建失败后，已创建的目录和数据库是否自动清理？
13. 代理商 Key 是否允许绑定多个代理站点？
14. 代理商利润倍率是否允许低于 `1.0`？
15. 平台售价变化后，代理站点价格是实时查询，还是定时同步到本地？
16. 代理站点是否允许对不同渠道设置不同利润倍率？

## 11. 推荐一期实现范围

为降低风险，建议一期按以下范围实现：

1. 管理后台新增代理商管理。
2. 代理商支持登录。
3. 代理商后台只开放“生成 Key”和“搭建代理站点”。
4. 代理商必须先生成启用状态的 Key，才允许搭建代理站点。
5. 代理站点创建时绑定代理商 Key，并使用该 Key 对接平台 API。
6. 搭建站点时复制 `/data/code/FanAPI` 到 `/data/code/{site_code}`。
7. 自动创建独立数据库名并写入 `config.yaml`。
8. 自动分配 Redis DB 并写入 `config.yaml`。
9. SMTP 信息由代理商填写并写入 `config.yaml`。
10. 代理商进价固定等于平台售价，代理商只能设置利润倍率。
11. 平台渠道真实进价只留在平台主站，不能同步到代理站点。
12. NATS 暂时复用同一个地址，但上线前必须确认 Worker 和 subject 是否支持站点隔离。
13. 搭建任务记录状态和失败原因，支持管理员查看。

## 12. 示例配置结果

代理商填写：

```text
网站名字：MID CODE
Logo URL：https://example.com/logo.png
SMTP Host：smtp.example.com
SMTP Port：465
SMTP User：admin@example.com
SMTP Password：<smtp-password>
SMTP From：MID CODE <admin@example.com>
```

系统生成：

```text
站点编码：midcode
代码目录：/data/code/midcode
数据库名：midcode_a1b2c3d4
Redis DB：1
绑定平台 API Key：sk-****abcd
```

目标目录 `config.yaml` 修改后示例：

```yaml
db:
  dbname: midcode_a1b2c3d4

redis:
  db: 1

nats:
  url: "nats://127.0.0.1:4222"

smtp:
  host: smtp.example.com
  port: 465
  user: admin@example.com
  password: "<smtp-password>"
  from: "MID CODE <admin@example.com>"

platform_api:
  base_url: "https://平台主站API地址"
  key: "<agent-api-key>"
  price_sync_enabled: true
```

价格示例：

```text
平台售价：0.80 CNY / 秒
代理商进价：0.80 CNY / 秒
利润倍率：1.70
代理站终端售价：1.36 CNY / 秒
```
