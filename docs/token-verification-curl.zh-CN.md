# Token 质量检测：curl 黑盒验证方案

本文档描述 HermesToken 的 Token 质量检测系统。当前支持 OpenAI 兼容协议和 Anthropic 兼容协议；检测方式为服务端执行 curl 命令，对本机 `/v1` 中转接口做真实黑盒请求；评分锚定到 [Artificial Analysis](https://artificialanalysis.ai/) 公开 P50；模型身份通过响应字段 + `system_fingerprint` 双层证据校验。

> 系统由三层校准互补保证质量：**Layer-1**（`go test TestCalibrationMatrix`）防算法回归；**Layer-2**（`scripts/run-calibration-e2e.sh`）防部署漂移；**Layer-3**（Prometheus 指标）防长尾异常。详见末尾「Prometheus 指标 → 与三层校准的关系」。

## 目标

检测模块要回答三个问题：

1. Token 是否真实可用。
2. Token 是否能调用用户预期的模型，并验证不被中转站偷换为低档模型。
3. Token 的基础质量如何，包括模型访问、流式能力、JSON 输出、延迟、首字节时间和输出速度。

黑盒 API 无法 100% 证明模型身份，但通过以下两层证据组合可显著提升中转站伪造成本：

- **Layer A — 响应模型字段一致性**（`model_identity` check）：
  - 从 `model_access` 的真实响应里读取 `observed_model`，与请求的 `claimed_model` 做一致性判断。
  - 对官方别名、日期版本、同系列同档位做宽松匹配。
  - 响应模型明显低于声明模型档位时标记 `suspected_downgrade=true`。
  - 输出 `identity_confidence` 并纳入评分。
  - 信号特点：成本低，但中转站可任意改写响应 JSON 的 `model` 字段（伪造难度低）。

- **Layer B — Seeded probe 复现性指纹**（`reproducibility` check）：
  - 对每个被测 OpenAI 系模型发两次 `temperature=0 + seed=42` 请求，比对 `system_fingerprint`（强信号）或响应内容哈希（弱信号）。
  - `system_fingerprint` 由上游模型提供商生成，中转站做模型替换时几乎无法伪造一致 fingerprint。
  - Anthropic Messages API 不支持 `seed`，自动跳过，不影响该 provider 评分。

要骗过整条链需要：响应 model 名 ✓ + `system_fingerprint` 一致 ✓ + 响应内容字节级一致 —— 中转站做不到这三件齐活。

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
- 如果使用用户 access token 调接口，需要额外带 `Authorization` 和 `HermesToken-User`，遵循项目现有用户 API 鉴权规则。

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
| `reproducibility` | 复现性指纹（同 seed/temperature=0 两次请求是否得到相同响应） |

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

任务一旦终止（`success` 或 `failed`），立即停止轮询。检测过程中后端会按 `provider × model × {availability, model_access, model_identity, stream_support, json_stability}` 的基础矩阵串行执行 curl 请求；OpenAI 兼容协议还会额外执行 `reproducibility` 两次同 seed 探针，Anthropic 会标记为 `skipped`。模型越多耗时越长，UI 上建议提示"正在检测中，可能需要 1~3 分钟"。

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
| `reproducibility` | 复现性指纹 | 同 seed/temperature=0 两次请求；优先比对 `system_fingerprint`，其次比对响应内容哈希 |

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
| 性能 | `performance` | 15 | 实测 TTFT / 输出速度 vs Artificial Analysis P50 的比值，无基线时回退到绝对耗时分档 |
| 流式输出能力 | `stream` | 10 | stream 检查是否成功 |
| JSON 输出稳定性 | `json` | 5 | JSON 检查是否成功 |

`models_list` 进入检查清单，但当前不单独占评分权重。

### performance 维度详解（v2+ / AA 基线）

`performance` 满分 15，由 TTFT 子分和输出速度子分各 7.5 分组成。当被测模型在 Artificial Analysis 缓存中能找到基线，且至少采集到一项流式性能数据时，按比值打分；否则回退到绝对耗时阶梯。

TTFT 子分（`measured_stream_ttft_ms / baseline_ttft_ms`，越低越好）：

| 比值 | 子分 |
| --- | ---: |
| ≤ 1.00 | 7.5 |
| ≤ 1.15 | 6.5 |
| ≤ 1.50 | 5.0 |
| ≤ 2.00 | 3.5 |
| ≤ 3.00 | 2.0 |
| > 3.00 | 1.0 |

输出速度子分（`measured_stream_tokens_ps / baseline_tokens_ps`，越高越好）：

| 比值 | 子分 |
| --- | ---: |
| ≥ 1.00 | 7.5 |
| ≥ 0.85 | 6.5 |
| ≥ 0.70 | 5.0 |
| ≥ 0.50 | 3.5 |
| ≥ 0.30 | 2.0 |
| < 0.30 | 1.0 |

多个模型时取每模型分数的算术平均后四舍五入到整数。

回退到绝对耗时阶梯时（`baseline_source = "fallback_absolute"`），仍沿用旧逻辑：

| 平均延迟 (`avg_latency_ms`) | 子分 |
| --- | ---: |
| ≤ 1500 ms | 15 |
| ≤ 3000 ms | 12 |
| ≤ 6000 ms | 9 |
| ≤ 10000 ms | 6 |
| > 10000 ms | 3 |

> 注意：Artificial Analysis 测试机部署在 GCP `us-central1-a`，TTFT 含网络 RTT。如果网关落在国内/香港，实测 TTFT 会系统性偏高，需要做地理校准（详见后文「Artificial Analysis 基线接入」）。

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
  "scoring_version": "v3",
  "baseline_source": "artificial_analysis",
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
  "models": [
    {
      "provider": "openai",
      "model_name": "gpt-4o",
      "available": true,
      "latency_ms": 920,
      "stream_ttft_ms": 480,
      "stream_tokens_ps": 72.5,
      "baseline": {
        "source": "artificial_analysis",
        "slug": "gpt-4o",
        "baseline_ttft_ms": 420,
        "baseline_tokens_ps": 88.5,
        "ttft_ratio": 1.143,
        "tps_ratio": 0.819
      }
    }
  ],
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
  "reproducibility": [
    {
      "provider": "openai",
      "model_name": "gpt-4o-mini",
      "consistent": true,
      "method": "system_fingerprint",
      "skipped": false,
      "message": "两次请求 system_fingerprint 一致"
    },
    {
      "provider": "anthropic",
      "model_name": "claude-3-5-haiku-latest",
      "consistent": false,
      "method": "",
      "skipped": true,
      "message": "Anthropic Messages API 不支持 seed 参数，跳过复现性检查"
    }
  ],
  "metrics": {
    "avg_latency_ms": 1120.5,
    "avg_ttft_ms": 450,
    "avg_tokens_per_second": 38.2,
    "aa_ttft_ratio_avg": 1.143,
    "aa_tps_ratio_avg": 0.819
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

新增字段语义：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `scoring_version` | string | 评分算法版本。历史无该字段的报告隐含为 `"v1"`；`"v2"` 引入 AA 基线；`"v3"` 加入复现性指纹对 stability/risks 的影响 |
| `baseline_source` | string | `artificial_analysis`（全部模型命中 AA 基线）/ `mixed`（部分命中）/ `fallback_absolute`（无命中，回退绝对阶梯）/ `none`（无任何性能数据） |
| `models[].stream_ttft_ms` | int | 流式检查测得的首 token 时间，毫秒 |
| `models[].stream_tokens_ps` | float | 流式检查估算的 tokens/s |
| `models[].baseline` | object | 该模型的 AA 基线及与实测的比值；命中时存在，否则缺省 |
| `models[].baseline.ttft_ratio` | float | 实测 TTFT / 基线 TTFT，**越低越好**，可直接用于前端展示 |
| `models[].baseline.tps_ratio` | float | 实测输出速度 / 基线输出速度，**越高越好** |
| `metrics.aa_ttft_ratio_avg` | float | 命中基线的所有模型 `ttft_ratio` 平均，便于一眼看出整体性能水平 |
| `metrics.aa_tps_ratio_avg` | float | 同上，输出速度比值平均 |
| `reproducibility[]` | array | 每个被测模型一条，记录复现性检查结果 |
| `reproducibility[].consistent` | bool | 两次相同 seed/temp=0 请求是否得到一致响应 |
| `reproducibility[].method` | string | 一致性判定方法：`system_fingerprint` / `system_fingerprint_changed` / `content_hash` / `content_diverged` / `insufficient_data` |
| `reproducibility[].skipped` | bool | 该 provider 不支持 seed（Anthropic）时为 `true`，本条不参与稳定性评分 |

`scoring_version` 同时落到 `token_verification_reports` 表的独立列，方便后续按版本筛选历史任务做趋势分析。

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

已落地的两层身份证据见上文「目标」一节。下一阶段可继续叠加更难伪造的信号：

- `behavior_fingerprint_score`：**未实现**——隐藏题库行为指纹得分（特定 prompt 引出模型特定行为）。
- `official_baseline_similarity`：**部分实现**——通过 Artificial Analysis 基线接入实现了**性能指标**（TTFT / 输出速度）的 P50 对照（详见「Artificial Analysis 基线接入」章节），但**未实现**输出文本相似度对照（需要官方权威输出参考集）。
- `long_context_score`：**未实现**——长上下文 needle-in-haystack 探针得分。
- `tool_call_score`：**未实现**——工具调用协议字段顺序与命名稳定性得分（跨家族转换时几乎必崩，是强指纹）。
- `multi_period_consistency`：**部分实现**——layer-2 e2e 校准矩阵（`scripts/run-calibration-e2e.sh`）支持每日 cron 跑同一组场景产出趋势数据，但还未做基于多次结果的方差告警。

按"伪造成本"排序，下一个最值得做的是 **`tool_call_score`**：工具调用 schema 是上游协议层产物，中转站做模型替换时跨家族 finish_reason / tool_use_id 命名差异几乎必然暴露。

## Artificial Analysis 基线接入

Token 校验的 `performance` 维度对接 [Artificial Analysis](https://artificialanalysis.ai/) 公开的 LLM 性能数据，用作"标杆"。基线只用于打分参照，不会替代我们自己的真实 curl 测量。

### 数据源

| 项 | 值 |
| --- | --- |
| 接口 | `GET https://artificialanalysis.ai/api/v2/data/llms/models` |
| 鉴权 | HTTP header `x-api-key: <AA_API_KEY>` |
| 单次返回 | 约 1 KB/模型 × 数十个主流模型，含 `slug`、`median_time_to_first_token_seconds`、`median_output_tokens_per_second` |
| 限流 | 1000 req / day |
| 测量位置 | GCP `us-central1-a`（含网络 RTT） |
| AA 自身聚合窗口 | 中位 P50，按 prompt 长度分组（详见 [methodology](https://artificialanalysis.ai/methodology/performance-benchmarking)） |

### 配置项（环境变量）

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `AA_API_KEY` | （空） | 必填，未配置时 `baseline_source` 永远是 `fallback_absolute` |
| `AA_BASELINE_ENABLED` | （空 → 取决于 key） | 显式 `false` 可在 key 已配置时强制关闭基线评分 |
| `AA_REFRESH_INTERVAL_HOURS` | `24` | 同步周期；最小 1 小时 |

### 同步与缓存

- 启动时立即拉取一次；之后按 `AA_REFRESH_INTERVAL_HOURS` 周期性刷新。
- 仅 master 节点跑同步任务（沿用 `common.IsMasterNode` 约定）。
- 命中 Redis 时落 key `token_verifier:aa_baseline`，TTL 14 天；未启用 Redis 则只放进程内存。
- 同步失败、API 5xx、限流时不会清空旧数据；缓存自然过期前继续生效。

### 模型名匹配

AA 用 `slug`（如 `gpt-4o`、`claude-3-5-haiku`）作为标识。匹配时复用 `canonicalModelName`：

- 大小写、`_`/`-` 归一化
- 去掉 `-latest` / `-preview` 后缀
- 去掉日期后缀（`gpt-4o-2024-05-13` → `gpt-4o`）

因此用户填 `gpt-4o-2024-05-13`、`gpt_4o`、`GPT-4o` 等变体都能命中同一个 AA 基线。命中失败的模型在该任务里走 fallback 阶梯。

### 失败回退路径

任意一个条件满足都会让 `performance` 走 `fallback_absolute`：

- AA 未配置或被关闭
- 同步从未成功，缓存为空
- 用户测的模型在 AA 列表中找不到（长尾模型常见）
- 流式 check 失败导致没有可用的实测 TTFT / tokens-per-second

混合场景（部分模型命中、部分未命中）走 `mixed`，命中部分用比值评分，未命中部分不影响最终的性能子分。

### 地理校准建议

AA 测试机在 GCP us-central1，国内/香港部署的网关测出的 TTFT 会系统性偏高（典型 +200~400 ms）。建议生产前跑一次校准基线：

1. 用直连官方 endpoint 的 token，从同一台部署机器上测主流模型（`gpt-4o`、`gpt-4o-mini`、`claude-3-5-haiku-latest`、`claude-opus-4-5`）。
2. 记录每个模型的 `aa_ttft_ratio_avg`，得到本地"网关固有偏移"基准。
3. 后续任何 token 的 `ttft_ratio` 都应该和该基准在 ±15% 内；显著高出说明上游有问题，不是基线偏差。

仓库提供了一键校准脚本 `scripts/calibrate-aa-baseline.sh`，需 `curl`、`jq`、`awk`。用法：

```bash
scripts/calibrate-aa-baseline.sh \
  --gateway https://api.example.com \
  --token-id 42 \
  --access-token <user_access_token> \
  --user-id 7
```

`access_token` 从 `GET /api/user/token` 获取；`user-id` 即当前登录用户的数字 id。脚本会创建一次校验任务、轮询完成、按模型打印 TTFT/TPS 实测/基线/比值，并给出该网关 region 推荐的"正常区间"和"异常告警"阈值。脚本退出码：`0` 校准成功，`1` 调用失败，`2` 命中 fallback（基线不可用）。

### 健康检查（手工）

AA 数据是否最新可以通过日志关键字 `AA baseline refreshed: models=<n>` 确认。如果连续多个刷新周期没看到这条，需检查：

- `AA_API_KEY` 是否正确（401 会出现在 warn 日志）
- 容器是否能出网到 `artificialanalysis.ai`
- 是否撞 1000 req/day 限流

## 复现性指纹检测（reproducibility）

`reproducibility` 是模型身份判定的"第二证据层"。`model_identity` 只看响应里的 `model` 字段，中转站可以任意改写；本检查通过两次相同请求的稳定性给出**中转站难以伪造**的旁证。

### 检测原理

对每个被测模型（OpenAI 系，Anthropic 跳过），后端串行发两次完全相同的 chat completion：

```jsonc
{
  "model": "<被测模型>",
  "messages": [{"role":"user","content":"Reply with this exact string and nothing else: STABLE_PING_9F3"}],
  "max_tokens": 32,
  "temperature": 0,
  "seed": 42,
  "stream": false
}
```

然后按以下优先级判定一致性：

| 优先级 | 判定 | `method` 取值 | 含义 |
| --- | --- | --- | --- |
| 1 | 两次都返回了 `system_fingerprint` 且相等 | `system_fingerprint` | 强一致：上游确认两次走的是同一模型同一配置 |
| 2 | 两次都返回了 `system_fingerprint` 但不等 | `system_fingerprint_changed` | **强不一致**：模型/路由/配置发生变化，自动加入 `risks` |
| 3 | 缺 `system_fingerprint`，但响应内容哈希相等 | `content_hash` | 弱一致：seed 生效，至少响应可复现 |
| 4 | 缺 `system_fingerprint`，响应内容哈希不等 | `content_diverged` | 弱不一致：seed 未真正生效或上游有抖动 |
| 5 | 两类信号都缺失 | `insufficient_data` | 数据不够，不下结论 |

### 信号强度对比

| 检测 | 中转站伪造难度 | 备注 |
| --- | --- | --- |
| `model_identity`（响应 model 字段） | 低 | 中转站可任意改写响应 JSON 的 `model` 字段 |
| `reproducibility` 内容哈希 | 中 | 中转站需缓存第一次响应才能保证两次一致；增加成本 |
| `reproducibility` `system_fingerprint` | 高 | 中转站若做模型替换，伪造一致 fingerprint 几乎不可能；除非完全照搬上游字段 |

二者结合后，要骗过整条链需要：响应 model 名 ✓ + system_fingerprint ✓ + 内容字节级一致 —— 中转站做不到这三件齐活。

### 跳过条件

- **Anthropic**：Messages API 不支持 `seed` 参数。结果以 `skipped=true` 标记，**不参与 stability 维度评分**（既不计入分子也不计入分母）。
- **请求失败**：第二次探测失败时整个 check 标 `success=false`，与其他失败 check 同等待遇。

### 评分影响

当前版本（v3）该检查不单独占评分维度，但通过两个间接路径影响总分：

1. **stability 维度**：成功的复现性检查给 stability 加分，失败的扣分。
2. **risks**：当 `method == system_fingerprint_changed` 时，自动在 `report.risks` 加一条 `"上游 system_fingerprint 在两次相同 seed 请求之间发生变化，疑似路由抖动或模型替换"`。

后续若实测显示 fingerprint-changed 的 false-positive 率足够低（< 5%），可考虑把该信号直接折入 `model_identity` 的 `identity_confidence`，进一步收紧降级判定。

### 成本

- 每被测的 OpenAI 系模型多 2 次 chat completion 调用（`max_tokens=32`，token 消耗极少）
- 单模型增加耗时约 1–3 秒
- Anthropic 模型零额外开销

## Prometheus 指标（observability）

Token 校验子系统通过 Prometheus 标准 exposition 格式暴露关键指标，用于做趋势分析、SLO 监控和异常告警。

### 端点

```
GET /api/performance/metrics
```

复用 `performance` 路由组，**鉴权使用 `RootAuth()`**。Prometheus job 配置时需要带 admin user access token 和 `HermesToken-User` header（与项目其他 admin API 一致）。

示例 `prometheus.yml`：

```yaml
scrape_configs:
  - job_name: hermestoken
    metrics_path: /api/performance/metrics
    scheme: https
    static_configs:
      - targets: ['gateway.example.com']
    authorization:
      type: Bearer
      credentials: <ADMIN_ACCESS_TOKEN>
    params:
      _: []
    # HermesToken-User 通过 relabel 注入
    relabel_configs:
      - target_label: __param_x   # placeholder
```

由于 `HermesToken-User` 是 HermesToken 自定义 header 不在 Prometheus 标准 scrape 选项里，建议用一个**反向代理**（nginx/traefik）在前面注入这个 header；或者部署一个内部 sidecar 把 `/metrics` 转发出来不带认证（仅在内网 access 受控时可行）。

### 指标列表

所有指标以 `hermestoken_token_verification_` 为前缀。

| 指标 | 类型 | 标签 | 含义 |
| --- | --- | --- | --- |
| `tasks_total` | Counter | `status` (`success` / `failed`) | 检测任务终态计数 |
| `tasks_by_grade_total` | Counter | `grade` (`S` / `A` / `B` / `C` / `D` / `Fail`) | 成功完成任务的等级分布 |
| `task_duration_seconds` | Histogram | `baseline_source` | 任务端到端耗时（s）；buckets: 1..300s |
| `downgrade_detected_total` | Counter | — | 累计 `suspected_downgrade=true` 的 (provider, model) 条数 |
| `aa_refresh_total` | Counter | `result` (`success` / `failed`) | AA 基线同步任务的执行结果 |
| `aa_baseline_models` | Gauge | — | 当前 AA 缓存中模型数；`-1` 表示无快照 |
| `aa_baseline_age_seconds` | Gauge | — | 距离上次 AA 同步的秒数；`-1` 表示无快照 |
| `aa_ttft_ratio` | Histogram | — | 实测 TTFT / AA 基线 TTFT 比值分布；越低越好；buckets: 0.5..10 |
| `aa_tps_ratio` | Histogram | — | 实测 tokens/s / 基线 tokens/s 比值分布；越高越好；buckets: 0.1..2.0 |

### 推荐告警规则

```yaml
groups:
  - name: hermestoken_token_verification
    rules:
      # AA 同步连续 2 个周期失败 (考虑默认 24h 间隔)
      - alert: HermestokenAARefreshFailing
        expr: increase(hermestoken_token_verification_aa_refresh_total{result="failed"}[2d]) >= 2
        for: 1h
        annotations:
          summary: "AA baseline 同步失败 ≥ 2 次"
          description: "检查 AA_API_KEY、出网到 artificialanalysis.ai 的连通性、是否撞 1000 req/day 限额"

      # AA 缓存超过 3 天未刷新
      - alert: HermestokenAABaselineStale
        expr: hermestoken_token_verification_aa_baseline_age_seconds > 3 * 24 * 3600
        for: 30m
        annotations:
          summary: "AA baseline 缓存陈旧 (>3 天)"

      # 任务失败率 > 30%（5 分钟窗口）
      - alert: HermestokenVerificationFailureRateHigh
        expr: |
          rate(hermestoken_token_verification_tasks_total{status="failed"}[5m])
          / on()
          rate(hermestoken_token_verification_tasks_total[5m]) > 0.3
        for: 10m
        annotations:
          summary: "Token 校验失败率 > 30%"

      # TTFT 比值 p90 偏离基线超过 2.0（可能上游路由问题）
      - alert: HermestokenAATTFTRatioHigh
        expr: |
          histogram_quantile(0.9,
            rate(hermestoken_token_verification_aa_ttft_ratio_bucket[15m])
          ) > 2.0
        for: 15m
        annotations:
          summary: "实测 TTFT 比 AA 基线慢 ≥ 2x（p90）"
          description: "注意：网关跨 region 时 baseline ≈ 1.2-1.5 是正常，单纯 > 2 也可能是地理偏差，需结合校准基线判断"

      # 降级事件率突增
      - alert: HermestokenDowngradeSpike
        expr: rate(hermestoken_token_verification_downgrade_detected_total[1h]) > 0.1
        for: 30m
        annotations:
          summary: "1h 内每分钟检测到 > 0.1 起模型降级"
```

### 关键 Grafana 面板建议

1. **任务量与成功率**：`rate(tasks_total{status="success"}[5m]) / rate(tasks_total[5m])`
2. **Grade 分布**：`sum by (grade) (rate(tasks_by_grade_total[1h]))` 堆叠面积图
3. **任务耗时 p50/p90/p99**：`histogram_quantile(0.9, rate(task_duration_seconds_bucket[5m]))`
4. **AA TTFT/TPS 比值热力图**：直接用 histogram 数据出 heatmap
5. **降级事件时序**：`increase(downgrade_detected_total[1h])` 柱状图
6. **AA baseline 健康**：`aa_baseline_age_seconds` 折线 + 阈值线 24h/72h

### 与三层校准的关系

| 层 | 工具 | 验证粒度 | 频率 |
| --- | --- | --- | --- |
| Layer-1 算法测试 | `go test TestCalibrationMatrix` | 输入→输出映射 | 每次 CI |
| Layer-2 端到端 | `scripts/run-calibration-e2e.sh` | 真上游+真打分 | 日级 cron |
| Layer-3 持续观测 | Prometheus metrics | 全量真实流量 | 实时（5–15s 抓取） |

三层互补：layer-1 防代码回归，layer-2 防部署漂移，layer-3 防长尾异常。生产建议同时启用 layer-2 和 layer-3：layer-2 给出"已知场景下基线"，layer-3 给出"真实负载下趋势"。

## 运行时性能调优

检测任务的执行采用**三层并行 + 单层串行**结构：

- **跨 (provider, model) 并行**（受限并发）— 默认 3 个模型同时跑
- **单模型内 5 个 check 串行**（identity 依赖 access 结果，stream/json/repro 共享 access 上下文，保留可读顺序）
- **复现性 check 内的 2 次 seeded probe 并行** — 单模型多省 ~1.5s

可通过环境变量调优：

| 变量 | 默认 | 范围 | 说明 |
| --- | --- | --- | --- |
| `TOKEN_VERIFIER_MODEL_CONCURRENCY` | `3` | `[1, 16]` | 单任务内同时跑几个模型；超出范围会被 clamp 到边界。增大可缩短 N-模型任务总耗时，但会瞬时给上游打更多并发。 |
| `TOKEN_VERIFIER_TASK_TIMEOUT_SEC` | `300` | `[60, 1800]` | 单任务总超时秒数。多模型 + 慢上游需要适当上调。 |

**预估耗时**（每 check ~2s 上游响应、`max_tokens` 都是 64 以下）：

| 模型数 | 串行原版 | 并发=3（默认） | 并发=10 |
| ---: | ---: | ---: | ---: |
| 1 | ~12s | ~12s | ~12s |
| 5 | ~60s | ~24s | ~12s |
| 10 | ~120s | ~40s | ~24s |

公式参考：`(provider × ceil(models / concurrency) × ~12s) + provider × 2s (models_list)`。

> 注意：上游慢/限流时实际值会拉长。如果调高了 concurrency，请同步上调 `TOKEN_VERIFIER_TASK_TIMEOUT_SEC`，避免边并发边超时。

### 调优建议

- **常规生产**：默认值即可
- **批量审计场景**（用户经常一次测 10+ 模型）：`TOKEN_VERIFIER_MODEL_CONCURRENCY=5` + `TOKEN_VERIFIER_TASK_TIMEOUT_SEC=600`
- **上游已限流**：`TOKEN_VERIFIER_MODEL_CONCURRENCY=1`（退化为串行，避免雪上加霜）
