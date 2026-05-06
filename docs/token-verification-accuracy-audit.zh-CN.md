# Token Verification 准确性审计流程

本文档用于把“检测功能所有检测项的准确性”从普通单元测试提升为真实模型输出上的可复核证据。

## 目标

硬审计通过时，必须同时满足：

- `full` profile 中每个检测项都有唯一审计路径。
- pass/fail 检测项在真实人工标注语料中至少有 1 个正样本和 1 个负样本。
- identity / submodel / neutral fingerprint 检测项进入 report-level 身份语料，并覆盖所有 required identity checks。
- informational 检测项在真实探针证据语料中至少有 1 个正样本和 1 个负样本。
- review-only 检测项有 judge 配置和官方 baseline 响应。
- 所有真实语料都带有人工复核元数据，且不是自动生成草稿。

## 覆盖清单

没有真实导出文件时，可以先查看 `full` profile 中每个检测项需要进入哪类硬审计证据：

```bash
go run ./scripts/token_verification_corpus \
  -mode coverage \
  -output /tmp/token-verification-accuracy-coverage.json
```

该输出只描述覆盖要求，不代表准确性已经通过。关键字段：

- `coverage_count`
- `coverage[].check_key`
- `coverage[].audit_path`
- `coverage[].evidence_requirement`

当前 `full` profile 的硬审计路径：

- `pass_fail_real_corpus`：47 个 pass/fail 检测项。每个 `check_key` 需要至少 1 个应通过真实样本和 1 个应失败真实样本。
- `identity_real_corpus`：49 个 neutral / identity / submodel fingerprint 检测项。它们不按单 case 做正负样本，而是复核整份 report-level identity assessment。
- `informational_real_corpus`：2 个 informational 检测项：`probe_channel_signature`、`probe_signature_roundtrip`。
- `review_only_judge`：3 个 review-only 检测项：`probe_zh_reasoning`、`probe_code_generation`、`probe_en_reasoning`。它们需要 judge 配置和 baseline 响应，不进入 pass/fail 真实语料。

查看某类语料具体覆盖哪些探针：

```bash
jq -r '.coverage[] | select(.audit_path=="pass_fail_real_corpus") | .check_key' \
  /tmp/token-verification-accuracy-coverage.json

jq -r '.coverage[] | select(.audit_path=="identity_real_corpus") | .check_key' \
  /tmp/token-verification-accuracy-coverage.json
```

## 生成草稿

先在页面或 API 跑一次真实目标模型的 token verification，并导出 direct probe JSON 或任务详情 JSON。

页面导出路径：

1. 打开 `/token-verification`。
2. 选择目标 provider / model / profile，运行真实检测。
3. 检测完成后点击“导出证据”，保存 `token-verification-evidence-*.json`。

当前页面导出的 direct probe JSON 会自动包含 `source_task_id` 和 `captured_at`；旧文件或外部 API envelope 缺少来源字段时，必须用下方参数补齐后再生成草稿。

先检查这份导出能生成多少草稿证据：

```bash
go run ./scripts/token_verification_corpus \
  -mode summary \
  -input /path/to/token-verification-evidence-openai-gpt-5-5-full.json
```

`source_ready` 必须为 `true`。如果 `missing_source` 出现 `source_task_id` 或 `captured_at`，用下方 `-source-task-id` / `-captured-at` 补齐；如果草稿 case 数明显偏少，先重跑真实检测，避免把覆盖不足的输出拿去人工复核。

然后生成四份草稿：

```bash
go run ./scripts/token_verification_corpus \
  -mode bundle \
  -input /path/to/direct-probe-or-task-detail.json \
  -output /tmp/token-verification-corpus \
  -description "real model capture YYYY-MM-DD provider/model"
```

输出文件：

- `/tmp/token-verification-corpus/labeled_probe_corpus_real.draft.json`
- `/tmp/token-verification-corpus/identity_assessment_corpus_real.draft.json`
- `/tmp/token-verification-corpus/informational_probe_corpus_real.draft.json`
- `/tmp/token-verification-corpus/probe_baseline.draft.json`

如果检测输出包含敏感值，生成草稿时传入 `-secret`：

```bash
go run ./scripts/token_verification_corpus \
  -mode bundle \
  -input /path/to/direct-probe-or-task-detail.json \
  -output /tmp/token-verification-corpus \
  -secret "sk-live-xxx,internal-hostname"
```

如果导出的是 direct probe JSON，且文件中没有任务 ID 或采集时间，生成草稿时补充来源元数据：

```bash
go run ./scripts/token_verification_corpus \
  -mode bundle \
  -input /path/to/direct-probe.json \
  -output /tmp/token-verification-corpus \
  -source-task-id "manual-capture-001" \
  -captured-at "2026-05-06T12:34:56Z"
```

`-captured-at` 必须是 RFC3339。任务详情 JSON 自带的 `task.id` 和 `task.created_at` 会优先保留，不会被这些参数覆盖。

## 人工复核

草稿默认带有：

```json
{
  "manual_review": {
    "status": "draft",
    "source": "detector_generated_draft"
  }
}
```

不能直接把草稿改名当成真实证据。人工复核每个 case 后，必须把三份真实语料改为：

```json
{
  "manual_review": {
    "status": "reviewed",
    "source": "real_model_output",
    "reviewed_by": "reviewer-name",
    "reviewed_at": "2026-05-06T00:00:00Z",
    "notes": "source task id / model / provider / review notes"
  }
}
```

复核要求：

- 不信任 `want_passed`、`want_error_code`、`want_identity_status` 等自动标签；它们只是当前检测器输出的初稿。
- 每个 case 都必须有 `source.provider`、`source.model`、`source.task_id`、`source.captured_at`。`captured_at` 必须是 RFC3339 时间，例如 `2026-05-06T00:00:00Z`。
- 对照真实模型响应、SSE、headers、usage、roundtrip 状态和 report-level identity 结论修正标签。
- 保留足够重放的字段，例如 `response_text`、`decoded`、`raw_sse`、`cache_header`、`context_levels`、`results`、`headers`、`raw_body`。
- 移除不可复核、空响应、上游错误、只含截断样本且无法判断的 case。
- 每个 pass/fail 与 informational 检测项至少保留 1 个应通过样本和 1 个应失败样本。

## 安装真实证据

人工复核后放到固定路径：

```bash
cp /tmp/token-verification-corpus/labeled_probe_corpus_real.draft.json \
  service/token_verifier/testdata/labeled_probe_corpus_real.json

cp /tmp/token-verification-corpus/identity_assessment_corpus_real.draft.json \
  service/token_verifier/testdata/identity_assessment_corpus_real.json

cp /tmp/token-verification-corpus/informational_probe_corpus_real.draft.json \
  service/token_verifier/testdata/informational_probe_corpus_real.json
```

同时保存 review-only baseline，例如：

```bash
export TOKEN_VERIFIER_PROBE_BASELINE_FILE=/tmp/token-verification-corpus/probe_baseline.draft.json
```

也可以在 audit CLI 里直接传入 baseline draft：

```bash
go run ./scripts/token_verification_corpus \
  -mode audit \
  -baseline /tmp/token-verification-corpus/probe_baseline.draft.json
```

审计临时合并目录时，不需要复制到仓库，可以传 `-base-dir`：

```bash
go run ./scripts/token_verification_corpus \
  -mode audit \
  -base-dir /tmp/token-verification-corpus-merged \
  -baseline /tmp/token-verification-corpus/probe_baseline.draft.json
```

baseline 必须至少覆盖：

- `zh_reasoning`
- `code_gen`
- `en_reasoning`

## Judge 配置

review-only 检测项需要 judge：

```bash
export TOKEN_VERIFIER_PROBE_JUDGE_BASE_URL="https://judge.example.com/v1"
export TOKEN_VERIFIER_PROBE_JUDGE_API_KEY="..."
export TOKEN_VERIFIER_PROBE_JUDGE_MODEL="judge-model"
export TOKEN_VERIFIER_PROBE_JUDGE_THRESHOLD="7"
```

不要把 judge key 或真实内部 endpoint 写入仓库。

也可以用 CLI 参数提供 judge 配置，并从环境变量读取 key，避免把 key 写进命令行或文件：

```bash
export TOKEN_VERIFIER_PROBE_JUDGE_API_KEY="..."

go run ./scripts/token_verification_corpus \
  -mode audit \
  -judge-base-url "https://judge.example.com/v1" \
  -judge-model "judge-model" \
  -judge-api-key-env TOKEN_VERIFIER_PROBE_JUDGE_API_KEY \
  -judge-threshold 7
```

## 硬审计

本地或 CI 使用：

```bash
go run ./scripts/token_verification_corpus -mode audit -require
```

通过时退出码为 0，且 JSON 中：

```json
{
  "passed": true
}
```

失败时退出码非 0，并在 JSON 中输出：

- `missing`
- `evidence_files[].case_count`
- `evidence_files[].manual_review_missing`
- `evidence_files[].case_source_missing`
- `evidence_files[].missing_coverage`
- `evidence_files[].false_positive`
- `evidence_files[].false_negative`
- `evidence_files[].invalid`

只想看下一轮要补哪些真实样本时，使用缺口摘要：

```bash
go run ./scripts/token_verification_corpus \
  -mode gaps \
  -baseline /tmp/token-verification-corpus/probe_baseline.draft.json \
  -output /tmp/token-verification-accuracy-gaps.json
```

关键字段：

- `missing_corpus_files`
- `missing_coverage`
- `target_check_keys`
- `target_check_keys_csv`
- `review_only_missing`
- `false_positive`
- `false_negative`
- `invalid`

`target_check_keys_csv` 可以直接喂给定向直连探针，避免每次重跑完整 `full` profile。也可以让直连探针直接读取 gaps 文件，并按审计路径选择目标清单：

```bash
set +x
read -rsp "HERMESTOKEN_PROBE_API_KEY: " HERMESTOKEN_PROBE_API_KEY; export HERMESTOKEN_PROBE_API_KEY; echo
read -rp "HERMESTOKEN_PROBE_BASE_URL: " HERMESTOKEN_PROBE_BASE_URL; export HERMESTOKEN_PROBE_BASE_URL
export HERMESTOKEN_PROBE_PROVIDER="anthropic"
export HERMESTOKEN_PROBE_MODEL="claude-opus-4-7"
export HERMESTOKEN_PROBE_PROFILE="full"
export HERMESTOKEN_PROBE_GAPS_FILE="/tmp/token-verification-accuracy-gaps.json"
export HERMESTOKEN_PROBE_GAPS_AUDIT_PATH="pass_fail_real_corpus"
export HERMESTOKEN_PROBE_OUTPUT="/tmp/token-verification-evidence-targeted.json"

go run ./scripts/token_verification_direct_probe
```

如果要用不同模型或配置补同一批正负样本，使用多模型环境变量并写入输出目录；不要给多模型采集配置单个 `HERMESTOKEN_PROBE_OUTPUT`，否则证据文件会被覆盖：

```bash
export HERMESTOKEN_PROBE_MODELS="claude-opus-4-7,claude-sonnet-4-6,claude-haiku-4-5"
export HERMESTOKEN_PROBE_OUTPUT_DIR="/tmp/token-verification-evidence-targeted-runs"

go run ./scripts/token_verification_direct_probe
```

脚本会为每个模型输出一个独立 JSON，例如 `01-anthropic-claude-opus-4-7.json`。后续分别执行 `-mode bundle`，人工复核成 `reviewed` 后再合并。

如果要混合不同 provider、base URL、client profile，使用 runs 文件。文件里只写环境变量名，不写密钥值：

```bash
go run ./scripts/token_verification_corpus \
  -mode runs-template \
  -baseline /tmp/token-verification-corpus/probe_baseline.draft.json \
  -output /tmp/token-verification-runs.json
```

```json
{
  "runs": [
    {
      "provider": "anthropic",
      "model": "claude-opus-4-7",
      "api_key_env": "ANTHROPIC_CAPTURE_KEY",
      "base_url_env": "ANTHROPIC_CAPTURE_BASE_URL",
      "client_profile": "claude_code",
      "gaps_audit_path": "pass_fail_real_corpus",
      "output_path": "/tmp/token-verification-evidence-targeted-runs/01-anthropic-opus.json"
    },
    {
      "provider": "openai",
      "model": "gpt-5.5",
      "api_key_env": "OPENAI_CAPTURE_KEY",
      "base_url_env": "OPENAI_CAPTURE_BASE_URL",
      "gaps_audit_path": "identity_real_corpus",
      "output_path": "/tmp/token-verification-evidence-targeted-runs/02-openai-gpt.json"
    }
  ]
}
```

```bash
set +x
read -rsp "ANTHROPIC_CAPTURE_KEY: " ANTHROPIC_CAPTURE_KEY; export ANTHROPIC_CAPTURE_KEY; echo
read -rp "ANTHROPIC_CAPTURE_BASE_URL: " ANTHROPIC_CAPTURE_BASE_URL; export ANTHROPIC_CAPTURE_BASE_URL
read -rsp "OPENAI_CAPTURE_KEY: " OPENAI_CAPTURE_KEY; export OPENAI_CAPTURE_KEY; echo
read -rp "OPENAI_CAPTURE_BASE_URL: " OPENAI_CAPTURE_BASE_URL; export OPENAI_CAPTURE_BASE_URL
export HERMESTOKEN_PROBE_RUNS_FILE="/tmp/token-verification-runs.json"
export HERMESTOKEN_PROBE_GAPS_FILE="/tmp/token-verification-accuracy-gaps.json"
export HERMESTOKEN_PROBE_GAPS_AUDIT_PATH="identity_real_corpus"

go run ./scripts/token_verification_direct_probe
```

identity 缺口里的 `family:openai` 需要 OpenAI 或 OpenAI-compatible provider 的真实输出；只用 Anthropic family 不能补这个维度。

每个 run 可以用 `gaps_audit_path` 从全局 `HERMESTOKEN_PROBE_GAPS_FILE` 选择目标探针，也可以用 `check_keys_csv` 明确覆盖目标清单。`check_keys_csv` 优先于 `gaps_audit_path`。

如果需要手工指定探针，`HERMESTOKEN_PROBE_CHECK_KEYS` 会优先于 gaps 文件：

```bash
export HERMESTOKEN_PROBE_CHECK_KEYS="$(jq -r '.target_check_keys_by_audit_path.pass_fail_real_corpus.csv' /tmp/token-verification-accuracy-gaps.json)"
```

`HERMESTOKEN_PROBE_API_KEY` 只从环境变量读取；不要写入仓库、日志或 shell history。跑完后继续用 `-mode bundle` 生成语料草稿并人工复核。

每轮人工复核后的真实语料可以先合并成候选文件，再用 hard audit 验证。合并命令只接受 `manual_review.status=reviewed` 且 `manual_review.source=real_model_output` 的输入：

```bash
go run ./scripts/token_verification_corpus \
  -mode merge-passfail \
  -description "Merged reviewed pass/fail corpus YYYY-MM-DD" \
  -output /tmp/token-verification-corpus-merged/service/token_verifier/testdata/labeled_probe_corpus_real.json \
  /tmp/token-verification-corpus-previous/service/token_verifier/testdata/labeled_probe_corpus_real.json \
  /tmp/token-verification-corpus-new/labeled_probe_corpus_real.reviewed.json
```

identity 和 informational 语料使用对应模式：

```bash
go run ./scripts/token_verification_corpus \
  -mode merge-identity \
  -description "Merged reviewed identity corpus YYYY-MM-DD" \
  -output /tmp/token-verification-corpus-merged/service/token_verifier/testdata/identity_assessment_corpus_real.json \
  /tmp/token-verification-corpus-previous/service/token_verifier/testdata/identity_assessment_corpus_real.json \
  /tmp/token-verification-corpus-new/identity_assessment_corpus_real.reviewed.json

go run ./scripts/token_verification_corpus \
  -mode merge-informational \
  -description "Merged reviewed informational corpus YYYY-MM-DD" \
  -output /tmp/token-verification-corpus-merged/service/token_verifier/testdata/informational_probe_corpus_real.json \
  /tmp/token-verification-corpus-previous/service/token_verifier/testdata/informational_probe_corpus_real.json \
  /tmp/token-verification-corpus-new/informational_probe_corpus_real.reviewed.json
```

合并后不要直接复制进仓库，先审计候选目录：

```bash
go run ./scripts/token_verification_corpus \
  -mode audit \
  -base-dir /tmp/token-verification-corpus-merged \
  -baseline /tmp/token-verification-corpus/probe_baseline.draft.json
```

强制测试入口：

```bash
TOKEN_VERIFICATION_REQUIRE_REAL_CORPUS=1 \
  go test -count=1 ./service/token_verifier \
  -run 'TestRequiredReal.*CorpusAudit|TestRequiredReviewOnlyProbeJudgeAudit' -v
```

普通回归仍需通过：

```bash
go test -count=1 ./...
node --test web/tests/token-verification-profiles.test.mjs
git diff --check
```

## 当前缺口解释

如果审计只显示以下缺口，说明代码门禁已经生效，但还没有安装真实证据：

- `service/token_verifier/testdata/labeled_probe_corpus_real.json`
- `service/token_verifier/testdata/identity_assessment_corpus_real.json`
- `service/token_verifier/testdata/informational_probe_corpus_real.json`
- judge config
- baseline config
- `zh_reasoning` / `code_gen` / `en_reasoning` baseline

此时不能宣称“所有检测项准确性已完成”，只能说明检测逻辑和审计门禁已经准备好，仍需真实模型输出与人工复核语料。
