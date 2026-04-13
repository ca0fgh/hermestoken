# Subscription Referral No-Compat Migration Design

## Goal

将当前“订阅返佣按分组”实现从“新旧兼容混合态”收敛为单一规则系统，彻底移除旧版空分组覆盖、旧版单值返佣配置、以及旧版默认视图语义。

本次目标是：

- 只保留基于 `subscription_plans.upgrade_group` 的返佣分组语义
- 明确规定 `upgrade_group == ""` 的订阅永不参与订阅返佣
- 删除旧版 `group=""` 返佣覆盖语义
- 删除旧版邀请人单值订阅返佣分配语义
- 删除旧版管理端和用户端的单值订阅返佣 API 兼容入口
- 通过一次性数据迁移和启动期校验，消除历史脏数据继续污染新逻辑的可能
- 调整本地/public Docker 启动脚本，默认重建镜像，避免代码与容器版本脱节

## Context

当前仓库已经实现了按分组订阅返佣的主体逻辑，但为了兼容旧数据和旧接口，仍保留了几条过渡逻辑：

- `subscription_referral_overrides.group = ""` 的旧覆盖数据仍可被读到
- 用户设置仍保留旧单值 `subscription_referral_invitee_rate_bps`
- 管理端与用户端接口仍保留 top-level 旧字段语义
- Docker 启动脚本默认 `docker compose up -d`，不会重建镜像

这些兼容层导致的直接问题已经在当前环境里出现过：

- UI 中出现与真实返佣分组不一致的“default”卡片
- 旧数据继续被兼容逻辑当成当前有效配置
- 新代码已合并但容器仍运行旧镜像
- PostgreSQL 方言与兼容路径交叉时更难定位问题

## Confirmed Product Decisions

用户已确认采用“不要兼容方案”的处理方式。以下规则已确认：

- 返佣分组的唯一业务来源是 `subscription_plans.upgrade_group`
- `upgrade_group == ""` 的订阅永远不返佣
- 旧版空分组返佣覆盖不再继续兼容展示/写入
- 旧版邀请人单值返佣分配不再作为长期数据模型保留
- 允许做一次性数据迁移，但不允许继续长期依赖兼容分支
- 本地/public Docker 启动默认应重建镜像，避免继续运行旧容器

## Out of Scope

本次不做：

- 自动智能推断“空分组旧覆盖”应该迁移到哪个真实分组（多分组场景禁止猜测）
- 对历史返佣流水做重新结算
- 新增复杂迁移后台页面
- 回滚到旧版单值返佣模型

## Approaches Considered

### Approach A: 继续兼容并逐步淡出

做法：

- 保留空分组覆盖
- 保留旧单值用户返佣字段
- 保留旧 API 顶层字段
- 继续通过兼容逻辑把旧数据映射成 `default`

问题：

- 规则长期双轨
- UI 和数据库容易继续出现“看起来像 default，实际不是 default”的错位
- 后续每次改动都要同时维护兼容分支

### Approach B: 破兼容，一次性迁移并删除旧语义（采用）

做法：

- 删除旧语义入口
- 明确一次性处理旧数据
- 保留少量启动期校验，只用于阻止脏数据继续存在，不用于继续兼容运行

优点：

- 规则单一
- 数据模型清晰
- 后续维护成本最低

代价：

- 需要一次性数据清理
- 旧客户端请求体会失效

## Decision

采用 **Approach B**。

原因：

- 这是唯一能彻底解决“旧数据继续污染新分组逻辑”的方案
- 当前问题已经证明兼容层会直接产生真实用户可见错误
- 长期维护双轨语义的成本和风险高于一次性迁移

## Target State

### 1. Group Semantics

- 唯一返佣分组来源：`subscription_plans.upgrade_group`
- `upgrade_group != ""` 才允许参与返佣
- 所有返佣设置、覆盖、用户分配都必须绑定到一个非空真实分组

### 2. Override Semantics

- `subscription_referral_overrides.group` 必须为非空
- 不再读取或写入 `group = ""`
- `GetLegacySubscriptionReferralOverrideByUserID` 这类旧语义 helper 应被删除或退化为仅用于迁移期校验

### 3. User Setting Semantics

- 只保留 `subscription_referral_invitee_rate_bps_by_group`
- 删除 `subscription_referral_invitee_rate_bps`
- 所有用户端保存必须显式带 `group`

### 4. API Semantics

- 管理端设置接口只返回/接受：
  - `enabled`
  - `groups`
  - `group_rates`
- 管理端用户覆盖接口只返回 grouped `groups[]`
- 用户端订阅返佣接口只返回 grouped `groups[]`
- 用户端 `PUT /api/user/referral/subscription` 必须带 `group`
- 所有旧 top-level 兼容字段从响应中移除

### 5. Docker Launch Semantics

- `scripts/local.py` 默认执行 `docker compose up -d --build`
- `scripts/public.py` 间接走同一重建逻辑
- 如果不想重建，应显式加跳过参数，而不是默认跳过

## Data Migration

### Migration Principle

不长期兼容旧数据，但允许一次性迁移。

### Migration Rules

1. `subscription_referral_overrides.group = ""`
   - 若系统只有一个真实返佣分组，则允许自动迁移到该分组
   - 若存在多个真实返佣分组，则不自动猜测；应记录告警并阻止启动，要求管理员先处理

2. 用户设置里的旧单值 `subscription_referral_invitee_rate_bps`
   - 若系统只有一个真实返佣分组，则迁移到该分组键下
   - 多分组场景下不自动猜测；记录告警并要求管理员处理

3. 迁移完成后
   - 删除旧字段/旧语义读取逻辑
   - 启动时若再次发现旧数据，直接报错，不进入正常服务

## Backend Changes

### Model Layer

调整：

- `SubscriptionReferralOverride.Group` 设为非空强约束
- 删除旧空分组 fallback helper
- `GetEffectiveSubscriptionReferralTotalRateBps(userID, group)` 只按非空分组走
- `ListSubscriptionReferralConfiguredGroups()` 继续返回：
  - `group_rates` 中已有的真实分组
  - 所有非空 `subscription_plans.upgrade_group`

### Controller Layer

调整：

- 删除 legacy/default 兼容顶层字段构造
- grouped 写接口只接受真实非空分组
- 删除“空 group 自动映射 default”的逻辑
- 对于已下线但仍被套餐引用的分组：
  - 只要它仍存在于 `subscription_plans.upgrade_group`，就允许读写返佣配置

### Startup Validation

新增启动期检查：

- 若仍存在 `subscription_referral_overrides.group = ""`
  - 单分组可自动迁移
  - 多分组直接失败
- 若仍存在旧用户单值返佣字段
  - 单分组可自动迁移
  - 多分组直接失败

目标是“迁移或失败”，而不是“继续兼容运行”。

## Frontend Changes

### Admin Settings Page

- 只使用后端 settings 接口返回的 `groups` + `group_rates`
- 不再依赖任何 top-level legacy 返佣率字段
- 页面不再构造虚拟 `default` 行

### Admin Override UI

- 只渲染 grouped `groups[]`
- 不再显示 legacy/default 顶层覆盖块

### User Invitation UI

- 只渲染 grouped `groups[]`
- 保存时必须带 `group`
- 不再支持无分组保存

## Error Handling

### Startup

- 发现无法自动迁移的旧数据时，直接失败并打印明确错误

### Runtime

- 对空 group 请求直接返回参数错误
- 对未知且非 plan-backed 的 group 继续返回“分组不存在”

## Testing Strategy

必须覆盖：

- 单分组场景下旧空分组 override 自动迁移
- 多分组场景下旧空分组 override 阻止启动
- 单分组场景下旧单值 invitee rate 自动迁移
- 多分组场景下旧单值 invitee rate 阻止启动
- 新 grouped API 不再接受旧无 group 用户写请求
- settings/local/public 启动脚本默认重建镜像

## Rollout

上线顺序：

1. 合入破兼容迁移逻辑
2. 先在现有数据上跑迁移检查
3. 处理无法自动迁移的数据
4. 再重启服务

上线完成标志：

- 库里不存在 `subscription_referral_overrides.group = ""`
- 不存在旧单值用户订阅返佣配置
- UI 中不再出现虚拟 `default`
- `scripts/public.py` / `scripts/local.py` 默认重建镜像
