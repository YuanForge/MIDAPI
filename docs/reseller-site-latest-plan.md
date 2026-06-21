# 代理商子站方案 - 最新执行版

## 1. 最终结论

一期不做主站多租户大改，也不改主站现有核心业务表。

采用以下方案：

```text
主站负责代理商管理 + 子站搭建记录
代理站独立部署
代理站独立数据库
代理站独立 Redis DB
共用一套 NATS 服务，但通过 namespace / stream / subject 隔离
```

这个方案对主站影响最小，隔离性最好，符合“代理站不能影响主站”的核心要求。

## 2. 一期范围

一期只做托管代理模式：

- 代理商由管理员创建，不开放公开注册。
- 一个代理商可以创建多个代理站。
- 代理商必须先创建 Key，才能创建代理站。
- 一个代理商 Key 可以绑定多个代理站。
- 代理站独立部署、独立数据库、独立 Redis DB。
- NATS 共用一套服务，但每个代理站使用独立 namespace。
- 代理站不能自己添加第三方渠道。
- 代理站只能使用主站同步过来的模型和价格。
- 代理商只能设置利润倍率，不能修改主站渠道进价和渠道配置。

## 3. 主站需要新增的能力

主站后台新增“代理商管理”。

管理员可以：

- 创建 / 禁用代理商。
- 查看代理商 Key。
- 查看代理商创建的代理站。
- 查看代理站绑定的目录、数据库、Redis DB、NATS namespace。
- 查看代理站搭建状态和失败原因。
- 处理失败任务、重试搭建或清理遗留资源。

主站不建议改这些核心表：

```text
users
api_keys
billing_transactions
payment_orders
llm_logs
tasks
```

只新增代理商管理相关表即可。

建议新增：

```text
resellers                    代理商
reseller_sites               代理站
reseller_site_build_jobs     搭建任务
reseller_site_key_bindings   Key 与代理站绑定关系
```

## 4. 代理站资源隔离

每个代理站独立一套运行资源。

| 资源 | 隔离方式 |
| --- | --- |
| 代码 | 从 `/data/code/FanAPI` 复制一份到新目录 |
| 数据库 | 每个代理站一个独立 PostgreSQL DB |
| Redis | 每个代理站使用独立 Redis DB |
| NATS | 共用一个 NATS 服务，但使用独立 namespace |
| SMTP | 代理商自己填写 |
| 品牌 | 代理商填写站名、Logo |
| 域名 | 用户先解析，我们人工配置宿主机 Nginx |

## 5. NATS 方案

NATS 服务可以共用，因为当前代码已经支持 namespace 隔离。

每个代理站配置独立 namespace，例如：

```yaml
nats:
  url: "nats://127.0.0.1:4222"
  namespace: "site_midcode_a1b2c3d4"
  task_stream: "TASKS_site_midcode_a1b2c3d4"
  task_subject: "site_midcode_a1b2c3d4.task.>"
  result_stream: "RESULTS_site_midcode_a1b2c3d4"
  result_subject: "site_midcode_a1b2c3d4.result.>"
```

隔离目标：

- 每个代理站有自己的任务流。
- 每个代理站有自己的结果流。
- 不同代理站之间的任务和结果不会串。
- 主站和代理站不会互相消费任务。

## 6. Redis 方案

主站继续使用 Redis DB 0：

```yaml
redis:
  db: 0
```

代理站从 DB 1 开始分配：

```text
第一个代理站：redis.db = 1
第二个代理站：redis.db = 2
第三个代理站：redis.db = 3
```

如果 Redis DB 超过默认 16 个，由运维扩容 Redis `databases` 数量，或新建 Redis 实例。

Redis DB 分配需要持久化记录，避免并发创建代理站时重复分配。

## 7. 代理站搭建流程

代理商点击“搭建代理站”后，系统执行以下流程：

1. 校验代理商状态。
2. 校验代理商是否已经创建可用 Key。
3. 填写代理站名称。
4. 填写 Logo URL，可为空。
5. 填写 SMTP 配置。
6. 设置默认利润倍率。
7. 系统生成站点编码。
8. 创建独立数据库。
9. 分配独立 Redis DB。
10. 生成独立 NATS namespace。
11. 从 `/data/code/FanAPI` 复制代码到目标目录。
12. 写入代理站自己的 `config.yaml`。
13. 初始化代理站数据库。
14. 自动启动代理站服务。
15. 记录搭建结果。

目标目录示例：

```text
/data/code/midcode
/data/code/midcode_a1b2c3d4
```

建议目录规则：

```text
/data/code/{site_code}_{8位随机英文数字}
```

## 8. 搭建失败清理

如果代理站创建失败，系统需要自动清理已经创建的资源。

需要清理：

```text
已创建的数据库
已复制的代码目录
已创建但未成功启动的 Compose 项目
已写入但未完成的构建任务状态
```

Redis DB 和 NATS namespace 不一定需要物理删除，但必须把分配状态回滚，或标记为可重新使用。

如果清理失败，需要在后台记录遗留资源，方便管理员人工处理。

## 9. 价格和渠道规则

一期代理站不能自己添加第三方渠道。

代理站只使用主站同步过来的模型和价格。

价格规则：

```text
代理商进价 = 主站平台售价
代理站终端售价 = 代理商进价 × 代理商利润倍率
```

代理商可以修改：

- 利润倍率。
- 站点展示价格。
- 站点名称。
- Logo。
- SMTP。

代理商不能修改：

- 主站真实渠道进价。
- 主站渠道配置。
- 上游 API 地址。
- 上游 API Key。
- 请求脚本。
- 响应脚本。
- 错误处理脚本。
- 第三方渠道。

## 10. 代理站后台需要单独限制

代理站虽然复制主站代码，但不能直接开放完整主站管理后台。

代理站后台的渠道管理需要改成：

```text
同步渠道 / 利润设置
```

允许：

- 查看主站同步过来的模型和渠道。
- 查看代理商进价。
- 设置全局默认利润倍率。
- 设置单个模型或渠道的利润倍率。
- 查看代理站终端售价。

禁止：

- 新增第三方渠道。
- 隐藏主站同步渠道。
- 修改主站同步价格。
- 查看或修改主站真实渠道进价。
- 修改上游 API 地址。
- 修改上游 API Key。
- 编辑请求 / 响应 / 错误脚本。

这些限制必须在后端接口层实现，不能只依赖前端隐藏按钮。

## 11. 域名和 Nginx

域名流程：

1. 代理商先把域名解析到服务器。
2. 我们人工配置宿主机 Nginx 和域名证书。
3. 代理站容器内部 Nginx 自动配置。
4. 代理站服务自动启动。

后期代理站更新暂时由我们人工处理：

```text
拉代码
build
执行 Docker Compose 更新
```

## 12. 配置生成规则

代理站的 `config.yaml` 需要由搭建流程自动生成。

关键配置包括：

```yaml
db:
  dbname: "midcode_a1b2c3d4"

redis:
  db: 1

nats:
  namespace: "site_midcode_a1b2c3d4"
  task_stream: "TASKS_site_midcode_a1b2c3d4"
  task_subject: "site_midcode_a1b2c3d4.task.>"
  result_stream: "RESULTS_site_midcode_a1b2c3d4"
  result_subject: "site_midcode_a1b2c3d4.result.>"

smtp:
  host: "smtp.example.com"
  port: 465
  user: "no-reply@example.com"
  password: "<由代理商填写>"
  from: "代理站名称 <no-reply@example.com>"

platform_api:
  base_url: "https://主站API地址"
  key: "<代理商生成的Key>"
  price_sync_enabled: true
```

本地 `config.yaml` 不放入仓库，生产环境使用自己的配置。

## 13. 与主站的关系

代理站通过代理商 Key 对接主站 API。

主站负责：

- 提供可售模型。
- 提供平台售价。
- 控制真实渠道和上游 Key。
- 监控价格变动。
- 给代理站提供实时价格查询 / 同步能力。

代理站负责：

- 自己的用户注册登录。
- 自己的用户充值和消费。
- 自己的品牌展示。
- 自己的利润倍率。
- 使用代理商 Key 调用主站 API。

## 14. 最终推荐

最终方案：

```text
不做主站多租户大改。
不改主站现有核心业务表。
保留代理站独立部署。
主站只新增代理商管理和搭建记录。
NATS 共用但 namespace 隔离。
Redis 使用独立 DB。
数据库每个代理站独立。
代理站通过代理商 Key 对接主站 API。
```

这个方案隔离性强、对主站影响小，适合当前阶段快速落地。

