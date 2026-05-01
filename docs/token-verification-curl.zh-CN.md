# Token 质量检测：curl 黑盒验证方案

本文档描述 HermesToken 的 Token 质量检测 MVP。当前版本只支持 OpenAI 兼容协议和 Anthropic 兼容协议，检测方式为服务端执行 curl 命令，对本机 `/v1` 中转接口做真实黑盒请求。

## 目标

检测模块要回答三个问题：

1. Token 是否真实可用。
2. Token 是否能调用用户预期的模型。
3. Token 的基础质量如何，包括模型访问、流式能力、JSON 输出、延迟、首字节时间和输出速度。

当前版本不做模型身份的强证明。黑盒 API 无法 100% 证明模型身份，只能通过协议一致性、响应模型名、能力边界和性能指纹给出风险判断。

当前已实现轻量模型身份检测：

- 从 `model_access` 的真实响应里读取 `observed_model`。
- 将用户请求的 `claimed_model` 与 `observed_model` 做一致性判断。
- 对官方别名、日期版本、同系列同档位做宽松匹配。
- 如果响应模型明显低于声明模型档位，标记 `suspected_downgrade=true`。
- 输出 `identity_confidence`，并纳入最终评分。

## 支持协议

当前支持：

- `openai`
- `anthropic`

不传 `providers` 时默认只检测 `openai`。

## 接口

接口统一返回：

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

### 创建检测任务

```http
POST /api/token_verification/tasks
```

请求体：

```json
{
  "token_id": 123,
  "providers": ["openai", "anthropic"],
  "models": ["gpt-4o-mini", "claude-3-5-haiku-latest"]
}
```

字段说明：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `token_id` | number | 是 | 当前用户自己的 Token ID |
| `providers` | string[] | 否 | `openai`、`anthropic`，为空时默认 `openai` |
| `models` | string[] | 否 | 要检测的模型列表，最多取 10 个 |

如果 `models` 为空：

- Token 开启模型限制时，默认取 Token 的模型限制列表前 5 个。
- 否则默认使用 `gpt-4o-mini`。
- Anthropic 协议下，如果默认模型仍是 `gpt-4o-mini`，检测器会自动替换成 `claude-3-5-haiku-latest`。

返回：

```json
{
  "success": true,
  "message": "",
  "data": {
    "id": 1,
    "user_id": 12,
    "token_id": 123,
    "token_name": "my-token",
    "models": ["gpt-4o-mini", "claude-3-5-haiku-latest"],
    "providers": ["openai", "anthropic"],
    "status": "pending",
    "score": 0,
    "grade": "",
    "fail_reason": "",
    "created_at": 1714550000,
    "started_at": 0,
    "finished_at": 0
  }
}
```

接口受 `CriticalRateLimit` 保护，短时间内重复触发会被限流。

常见错误响应：

| 场景 | 返回 `message` |
| --- | --- |
| `token_id` 缺失或非正整数 | `请选择要检测的 Token` |
| `token_id` 不属于当前用户 | `record not found` 类（来自 GORM） |
| 命中限流 | 由限流中间件返回 |

返回 HTTP 状态恒为 `200`，调用方需以 `success` 字段为准。

### 查询检测任务列表

```http
GET /api/token_verification/tasks
```

支持项目通用分页参数：

- `p`：页码，从 1 开始，默认 1
- `page_size`：每页条数，默认 `ItemsPerPage`，**上限 100**，超过会被截断

列表按任务 `id` 倒序返回，仅展示当前用户自己的任务。

返回分页结构：

```json
{
  "success": true,
  "message": "",
  "data": {
    "page": 1,
    "page_size": 20,
    "total": 100,
    "items": []
  }
}
```

### 查询检测任务详情

```http
GET /api/token_verification/tasks/:id
```

`:id` 为创建任务时返回的 `id`。仅可查询当前用户自己的任务，他人任务返回 `record not found`。

任务在 `pending` 或 `running` 状态时，`results` 可能为空数组，`report` 为 `null`。仅 `success` 状态下才会有完整聚合报告。

返回内容包含：

- `task`：任务状态
- `results`：原始检测结果
- `report`：汇总报告

返回示例：

```json
{
  "success": true,
  "message": "",
  "data": {
    "task": {
      "id": 1,
      "user_id": 12,
      "token_id": 123,
      "token_name": "my-token",
      "models": ["gpt-4o-mini"],
      "providers": ["openai"],
      "status": "success",
      "score": 86,
      "grade": "A",
      "fail_reason": "",
      "created_at": 1714550000,
      "started_at": 1714550001,
      "finished_at": 1714550032
    },
    "results": [],
    "report": {}
  }
}
```

### 查询检测报告

```http
GET /api/token_verification/reports/:id
```

`:id` 是检测任务 ID（即创建任务时返回的 `id`），不是独立的报告 ID。

如果任务尚未完成或报告尚未生成，返回：

```json
{ "success": false, "message": "检测报告尚未生成" }
```

仅当任务 `status === "success"` 时才会有可用报告。如果只是想拿到任务最新状态 + 完整报告，建议使用「查询检测任务详情」接口轮询，本接口仅在"已知任务已完成、只想拿报告"时使用。

返回示例：

```json
{
  "success": true,
  "message": "",
  "data": {
    "task": {},
    "report": {}
  }
}
```

## 前端 API 接入说明

前端接入流程：

1. 用户选择一个自己的 Token。
2. 用户选择检测协议：`openai`、`anthropic`。
3. 用户选择要检测的模型。
4. 前端调用创建检测任务接口。
5. 后端异步执行检测。
6. 前端轮询任务详情。
7. 当任务状态为 `success` 或 `failed` 时停止轮询。
8. 展示检测清单、原始结果、指标汇总和最终评级。

推荐交互：

- 创建任务后立即进入检测详情页。
- 每 2 秒轮询一次 `GET /api/token_verification/tasks/:id`。
- 状态为 `pending` 或 `running` 时展示检测中。
- 状态为 `success` 时展示完整报告。
- 状态为 `failed` 时展示 `task.fail_reason`。
- `report.final_rating` 放在顶部作为总览。
- `report.checklist` 放在详情区域展示每项通过或失败。
- `report.models` 展示每个模型的可用性。
- `report.metrics` 展示延迟、首 token 时间、输出速度。
- `report.model_identity` 展示声明模型、响应模型、身份置信度、疑似降级。
- `report.risks` 有内容时展示风险提示。

前端展示优先级：

| 展示区域 | 推荐字段 | 说明 |
| --- | --- | --- |
| 顶部总览 | `report.final_rating` | 分数、等级、结论、风险 |
| 检查清单 | `report.checklist` | 每项检测通过/失败、延迟、错误 |
| 模型可用性 | `report.models` | 每个 provider + model 的可用状态 |
| 模型身份 | `report.model_identity` | 响应模型是否匹配声明模型 |
| 性能指标 | `report.metrics` | 平均延迟、平均 TTFT、平均输出速度 |
| 原始结果 | `results` | 调试和审计用 |

前端不需要让用户输入完整 Token Key，只需要传用户已有的 `token_id`。

认证说明：

- 浏览器前端使用当前登录态即可。
- 请求需要携带 cookie/session。
- 如果使用用户 access token 调接口，需要额外带 `Authorization` 和 `New-Api-User`，遵循项目现有用户 API 鉴权规则。

任务状态：

| 状态 | 说明 |
| --- | --- |
| `pending` | 等待执行 |
| `running` | 正在检测 |
| `success` | 检测完成 |
| `failed` | 检测失败 |

原始检测结果 `results` 每一条对应一个具体检测项：

```json
{
  "id": 1,
  "task_id": 1,
  "provider": "openai",
  "check_key": "model_identity",
  "model_name": "gpt-4o-mini",
  "claimed_model": "gpt-4o-mini",
  "observed_model": "gpt-4o-mini-2024-07-18",
  "identity_confidence": 90,
  "suspected_downgrade": false,
  "success": true,
  "score": 90,
  "latency_ms": 0,
  "ttft_ms": 0,
  "tokens_ps": 0,
  "error_code": "",
  "message": "响应模型名与声明模型属于同一官方别名或日期版本",
  "raw": "{\"claimed_model\":\"gpt-4o-mini\",\"observed_model\":\"gpt-4o-mini-2024-07-18\",\"identity_confidence\":90,\"suspected_downgrade\":false,\"identity_method\":\"response_model_consistency\"}",
  "created_at": 1714550004
}
```

`check_key` 枚举：

| `check_key` | 说明 |
| --- | --- |
| `models_list` | 模型列表接口 |
| `availability` | Token 基础可用性 |
| `model_access` | 模型调用可用性 |
| `model_identity` | 模型身份一致性 |
| `stream_support` | 流式输出能力 |
| `json_stability` | JSON 输出稳定性 |

### 前端调用示例

下面是最小可用的浏览器调用片段，仅作参考，实际接入建议使用项目内 `web/src/helpers/api.js` 的 axios 实例以保持鉴权与拦截器一致。

```js
// 1. 创建检测任务
const create = await fetch('/api/token_verification/tasks', {
  method: 'POST',
  credentials: 'include', // 必须，携带登录 cookie
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    token_id: 123,
    models: ['gpt-4o-mini', 'gpt-4o'], // 可选,最多 10 个
    providers: ['openai'],             // 可选,默认 ['openai']
  }),
}).then(r => r.json());

if (!create.success) throw new Error(create.message);
const taskId = create.data.id;

// 2. 轮询任务详情直到结束
async function pollTask() {
  const res = await fetch(`/api/token_verification/tasks/${taskId}`, {
    credentials: 'include',
  }).then(r => r.json());
  if (!res.success) throw new Error(res.message);

  const { task, report } = res.data;
  if (task.status === 'success') return { task, report };
  if (task.status === 'failed') throw new Error(task.fail_reason || '检测失败');

  await new Promise(r => setTimeout(r, 1500));
  return pollTask();
}

const { task, report } = await pollTask();
// 用 report.final_rating / report.dimensions / report.model_identity 渲染 UI
```

字段使用要点：

- 创建任务时**只传 `token_id`，不传明文 Token Key**。明文 Token 由后端按 `token_id` + 当前用户 ID 自行查库。
- `providers` 仅接受 `openai` 和 `anthropic`，其它值会被静默过滤；过滤后为空时回退为 `["openai"]`。
- `models` 超过 10 个会截断；不传时按 Token 的 `ModelLimits` 自动取前 5 个，仍为空则回退为 `gpt-4o-mini`。
- 轮询期间 `report` 字段可能为 `null`，渲染前需判空。

### 轮询节奏与超时建议

| 项 | 建议值 |
| --- | --- |
| 轮询间隔 | 1.5 ~ 2 秒一次 |
| 前端兜底超时 | 3 分钟未完成提示用户超时 |
| 后端硬超时 | 5 分钟（任务 ctx 上限），超时后任务会被标记 `failed` |

任务一旦终止（`success` 或 `failed`），立即停止轮询。检测过程中后端会按 `provider × model × {availability, model_access, model_identity, stream_support, json_stability}` 的矩阵串行执行 curl 请求，模型越多耗时越长，UI 上建议提示"正在检测中，可能需要 1~3 分钟"。

### 错误响应汇总

所有接口在出错时 HTTP 状态仍为 `200`，需通过 `success === false` 判断。常见 `message` 文本：

| 场景 | 来源接口 | `message` |
| --- | --- | --- |
| `token_id` 缺失或非正 | `POST /tasks` | `请选择要检测的 Token` |
| token 不属于当前用户 | `POST /tasks`、`GET /tasks/:id`、`GET /reports/:id` | `record not found` |
| 任务 ID 非法 | `GET /tasks/:id`、`GET /reports/:id` | `无效的检测任务ID` |
| 任务尚未完成 | `GET /reports/:id` | `检测报告尚未生成` |
| 命中创建任务限流 | `POST /tasks` | 由 `CriticalRateLimit` 中间件返回 |

任务自身失败（已创建、但执行过程中报错）不会通过上面的接口错误抛出，而是在 `task.status === "failed"` + `task.fail_reason` 中体现，例如：

- `token verifier base url is empty`：未配置 `TOKEN_VERIFIER_BASE_URL` 且系统 `ServerAddress` 为空。
- `curl failed: <stderr>`：服务器执行 curl 出错（curl 不存在 / 网络异常等）。
- DB 写入失败：极少见，通常表示后端存储异常。

## 内部 curl 验证方式

检测器通过 `TOKEN_VERIFIER_BASE_URL` 指定目标服务地址。未配置时使用系统 `ServerAddress`。

推荐生产配置：

```bash
TOKEN_VERIFIER_BASE_URL=http://127.0.0.1:3000
```

### OpenAI 兼容模型列表

```bash
curl -sS --no-buffer --max-time 40 \
  -X GET "$TOKEN_VERIFIER_BASE_URL/v1/models" \
  -H "Authorization: Bearer sk-xxx"
```

### OpenAI 兼容普通请求

```bash
curl -sS --no-buffer --max-time 40 \
  -X POST "$TOKEN_VERIFIER_BASE_URL/v1/chat/completions" \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  --data-binary '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Reply with exactly: ok"}],
    "max_tokens": 64,
    "stream": false
  }'
```

### OpenAI 兼容流式请求

```bash
curl -sS --no-buffer --max-time 40 \
  -X POST "$TOKEN_VERIFIER_BASE_URL/v1/chat/completions" \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  --data-binary '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Count from 1 to 20 separated by spaces."}],
    "max_tokens": 64,
    "stream": true
  }'
```

### Anthropic 兼容模型列表

```bash
curl -sS --no-buffer --max-time 40 \
  -X GET "$TOKEN_VERIFIER_BASE_URL/v1/models" \
  -H "x-api-key: sk-xxx" \
  -H "anthropic-version: 2023-06-01"
```

### Anthropic 兼容普通请求

```bash
curl -sS --no-buffer --max-time 40 \
  -X POST "$TOKEN_VERIFIER_BASE_URL/v1/messages" \
  -H "x-api-key: sk-xxx" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  --data-binary '{
    "model": "claude-3-5-haiku-latest",
    "max_tokens": 64,
    "messages": [{"role": "user", "content": "Reply with exactly: ok"}]
  }'
```

### Anthropic 兼容流式请求

```bash
curl -sS --no-buffer --max-time 40 \
  -X POST "$TOKEN_VERIFIER_BASE_URL/v1/messages" \
  -H "x-api-key: sk-xxx" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  --data-binary '{
    "model": "claude-3-5-haiku-latest",
    "max_tokens": 64,
    "stream": true,
    "messages": [{"role": "user", "content": "Count from 1 to 20 separated by spaces."}]
  }'
```

## 检查清单

当前 `checklist` 包含以下检查项：

| `check_key` | 检查项 | 说明 |
| --- | --- | --- |
| `models_list` | 模型列表接口 | 验证 `/v1/models` 是否可访问 |
| `availability` | Token 基础可用性 | 用第一个模型发起真实请求 |
| `model_access` | 模型调用可用性 | 对每个指定模型分别请求 |
| `model_identity` | 模型身份一致性 | 对比请求模型与响应模型名 |
| `stream_support` | 流式输出能力 | 验证 SSE `data:` 流是否有数据块 |
| `json_stability` | JSON 输出稳定性 | 要求模型返回 JSON 对象 |

每个检查项返回：

```json
{
  "provider": "openai",
  "check_key": "model_identity",
  "check_name": "模型身份一致性",
  "model_name": "gpt-4o-mini",
  "claimed_model": "gpt-4o-mini",
  "observed_model": "gpt-4o-mini-2024-07-18",
  "identity_confidence": 90,
  "suspected_downgrade": false,
  "passed": true,
  "status": "passed",
  "score": 90,
  "error_code": "",
  "message": "响应模型名与声明模型属于同一官方别名或日期版本"
}
```

## 评分维度

总分 100 分：

| 维度 | 字段 | 权重 | 说明 |
| --- | --- | ---: | --- |
| Token 基础可用性 | `availability` | 20 | 第一个模型请求是否成功 |
| 模型调用可用性 | `model_access` | 20 | 指定模型成功率 |
| 模型身份一致性 | `model_identity` | 15 | 响应模型与声明模型的一致性置信度 |
| 稳定性 | `stability` | 15 | 所有检查项成功率 |
| 性能 | `performance` | 15 | 平均响应耗时分档评分 |
| 流式输出能力 | `stream` | 10 | stream 检查是否成功 |
| JSON 输出稳定性 | `json` | 5 | JSON 检查是否成功 |

`models_list` 进入检查清单，但当前不单独占评分权重。

## 最终评级

| 等级 | 分数 | 说明 |
| --- | ---: | --- |
| S | 90-100 | 优质 Token，适合生产调用 |
| A | 80-89 | 稳定可用，适合日常调用 |
| B | 65-79 | 普通可用，存在轻微质量波动 |
| C | 50-64 | 可用但风险较高，建议谨慎使用 |
| D | 1-49 | 质量较差，不建议高频使用 |
| Fail | 0 | 核心检测未通过 |

## 报告结构

```json
{
  "score": 88,
  "grade": "A",
  "conclusion": "稳定可用，适合日常调用",
  "dimensions": {
    "availability": 20,
    "model_access": 18,
    "model_identity": 13,
    "stability": 13,
    "performance": 9,
    "stream": 10,
    "json": 5
  },
  "checklist": [],
  "models": [],
  "model_identity": [
    {
      "provider": "openai",
      "claimed_model": "gpt-4o-mini",
      "observed_model": "gpt-4o-mini-2024-07-18",
      "identity_confidence": 90,
      "suspected_downgrade": false,
      "message": "响应模型名与声明模型属于同一官方别名或日期版本"
    }
  ],
  "metrics": {
    "avg_latency_ms": 1120.5,
    "avg_ttft_ms": 450,
    "avg_tokens_per_second": 38.2
  },
  "risks": [],
  "final_rating": {
    "score": 88,
    "grade": "A",
    "conclusion": "稳定可用，适合日常调用",
    "dimensions": {},
    "risks": []
  }
}
```

## 当前模型身份检测原理

当前实现是第一阶段轻量方案，不额外增加请求成本：

1. `model_access` 请求成功后，解析响应 JSON。
2. 优先读取响应顶层 `model` 字段作为 `observed_model`。
3. 用户请求的模型作为 `claimed_model`。
4. 对两个模型名做标准化：
   - 转小写。
   - 下划线转中划线。
   - 去掉 `-latest`、`-preview`。
   - 去掉日期版本后缀，例如 `20241022` 或 `2024-07-18`。
5. 匹配规则：
   - 完全一致：`identity_confidence=95`。
   - 官方别名或日期版本一致：`identity_confidence=90`。
   - 同系列同档位：`identity_confidence=80`。
   - 响应档位高于声明档位：`identity_confidence=85`。
   - 响应档位低于声明档位：`identity_confidence=35`，`suspected_downgrade=true`。
   - 系列不一致：`identity_confidence=25`，`suspected_downgrade=true`。
   - 响应没有模型名：`identity_confidence=50`，不直接判定降级。

当前档位启发式：

- OpenAI：`gpt-5` > `gpt-4.5` > `gpt-4.1` > `gpt-4o` > `gpt-4o-mini` > `gpt-4o-nano` > `gpt-3.5`。
- Anthropic：`opus` > `sonnet` > `haiku`。

注意：这个结果是黑盒风险判断，不是法律意义上的强证明。强证明需要官方返回可验证签名、供应商审计日志，或在上游网关侧保存真实路由证据。

## 真实模型身份验证后续方案

后续要进一步判断“是否为预期模型”，建议新增：

- `behavior_fingerprint_score`：隐藏题库行为指纹得分。
- `official_baseline_similarity`：与官方基准输出的相似度。
- `long_context_score`：长上下文能力得分。
- `tool_call_score`：工具调用协议稳定性得分。
- `multi_period_consistency`：多时段复测一致性。

下一阶段可加入隐藏题库、官方基准输出对照、长上下文探针、工具调用探针和多时段复测。
