# Two-Level Subscription Referral Design

## Goal

为 `hermestoken` 的订阅体系设计一套新的“两级订阅返佣”方案，只保留：

- `直推`
- `团队`

本次目标是：

- 单独定义 `直推 + 团队` 的订阅返佣规则
- 只复用现有订阅套餐分组 `upgrade_group` 作为配置入口
- 不继续沿用旧的“邀请人 / 被邀请人”二分账语义
- 让新结算账本、触发规则、历史快照彼此独立
- 明确普通用户、直推用户、团队用户购买订阅时的不同返佣结果

## Context

当前项目已经有一套旧版订阅返佣实现：

- 订单支付成功后进入订阅返佣结算
- 账本核心是 `subscription_referral_records`
- 受益角色仍是 `inviter / invitee`
- 返佣率主要依赖：
  - 管理员给邀请人的分组 override
  - 邀请人自己设置“分给被邀请人的比例”

这套模型适合“邀请人让利给被邀请人”的二分账逻辑，但不适合本次要做的新规则：

- 返佣受益资格由身份决定，而不是由普通邀请关系自然继承
- 普通用户不能拿订阅返佣
- 团队直邀用户时只给该团队本人结算
- 付款用户自身身份会影响是否继续向上结算团队差额
- 新规则不再需要邀请人给被邀请人手动分账

因此，本次方案应被视为一个新的“订阅两级返佣模型”，而不是对旧模型的小修补。

## Confirmed Product Decisions

以下规则已与用户确认：

- 只对真实支付成功的订阅订单结算返佣
- 返佣方案按订阅套餐的 `upgrade_group` 生效
- 新方案只保留两个返佣身份：
  - `direct`
  - `team`
- 用户实际可拥有三种身份状态：
  - `normal`
  - `direct`
  - `team`
- 用户返佣身份是全局唯一，由后台指定
- 每个用户只能有 `1` 个直接邀请人
- 邀请链允许混排：
  - `direct -> team`
  - `team -> direct`
  - `team -> team`
  - `direct -> direct`
- 能不能拿订阅返佣，只看受益人身份是不是 `direct` 或 `team`
- `normal` 用户即使邀请了别人，也不能拿订阅返佣
- 付款用户本人不能拿自己这笔订阅返佣，返佣只发给上级受益节点
- 如果付款用户的直接邀请人是 `normal`，整笔订单不返佣，且不允许跳过该普通用户继续向上找返佣人
- 如果付款用户的直接邀请人是 `team`：
  - 不触发返佣链
  - 只给这个 `team` 本人结算
- 如果付款用户的直接邀请人是 `direct`：
  - 最近这个 `direct` 拿直推返佣
  - 是否继续给上层 `team` 返佣，取决于付款用户身份
- 只有当付款用户本身是 `normal`，且其直接邀请人是 `direct` 时，才允许继续向上计算团队差额返佣
- 如果付款用户本身已经是 `direct` 或 `team`，即使其直接邀请人是 `direct`，也只给最近这个 `direct` 结算，不再向上给团队返佣
- 历史订单结算结果需要快照固化，后续改邀请关系、改身份、改返佣配置都不影响历史订单

## Out of Scope

本次方案不包含：

- `项目` 级别返佣
- 旧版“邀请人分给被邀请人”的自助分账规则迁移
- 邀请用户独立返佣覆盖迁移到新模型
- 注册奖励、首单奖励等非订阅返佣
- 推广排行榜、经营分析大盘
- 自动跳过普通用户继续向上找返佣节点
- 一次性重写所有现有邀请返佣前台页面

## Approaches Considered

### Approach A: 在旧二分账模型上继续扩展

做法：

- 保留旧 `inviter / invitee` 账本
- 继续追加 `normal / direct / team` 判断
- 在原结算函数内叠加新的身份条件和差额逻辑

问题：

- 旧模型的核心语义是“邀请人 / 被邀请人分账”，不是“身份驱动返佣”
- 普通用户过滤、团队直邀单点结算、付款用户身份限制等规则会把旧结算逻辑绕乱
- 审计、退款、统计会长期混杂两套语义

### Approach B: 新两级模型（采用）

做法：

- 单独定义 `直推 + 团队` 的订阅返佣规则
- 只复用现有 `upgrade_group` 作为配置入口
- 结算账本和规则独立表达

优点：

- 规则和账本语义一致
- 便于分阶段灰度迁移
- 不会继续污染旧版订阅返佣实现
- 便于后续继续扩展身份驱动型返佣

代价：

- 需要新增方案表、批次表、明细表
- 需要在订阅成功主流程里接入新的结算引擎

### Approach C: 只结算最近一个合格上级

做法：

- 不引入团队差额池
- 无论何种身份链，始终只给最近一个有资格的上级结算

优点：

- 最简单

问题：

- 无法满足“普通用户被直推邀请时，上层团队还要拿差额”的业务要求
- 失去团队身份的经营激励意义

## Decision

采用 **Approach B**。

原因：

- 它是唯一既能满足当前两级业务规则，又不继续污染旧二分账模型的方案
- 它保留了 `upgrade_group` 这个现有订阅入口，避免额外制造新的返佣维度
- 它能把“谁有资格拿返佣”和“订单如何向上扩散”拆开处理，产品解释更稳定

## Business Model

### Identity Model

每个用户都有一个全局返佣身份：

- `normal`
- `direct`
- `team`

含义：

- `normal`
  - 普通用户
  - 可以购买订阅
  - 可以邀请别人
  - 但自己不能成为订阅返佣受益人
- `direct`
  - 直推身份用户
  - 可以成为订阅返佣受益人
- `team`
  - 团队身份用户
  - 可以成为订阅返佣受益人

### Referral Eligibility

一笔订阅订单是否有返佣入口，先看付款用户的直接邀请人：

1. 直接邀请人不存在
   - 不返佣
2. 直接邀请人是 `normal`
   - 不返佣
   - 不允许跳过这个普通用户继续向上找 `direct / team`
3. 直接邀请人是 `team`
   - 进入返佣结算
   - 但只给这个 `team` 本人结算
4. 直接邀请人是 `direct`
   - 进入返佣结算
   - 最近这个 `direct` 一定拿直推返佣
   - 是否继续向上给 `team` 结算，取决于付款用户身份

### Trigger Modes

新模型下，订单只会进入以下三种结算模式之一。

#### Mode 1: `team_direct`

条件：

- 付款用户的直接邀请人身份是 `team`

结果：

- 不触发返佣链
- 只给这个直接邀请用户的 `team` 本人结算
- 更上层节点全部不参与

#### Mode 2: `direct_only`

条件：

- 付款用户的直接邀请人身份是 `direct`
- 且付款用户身份是：
  - `direct`
  - 或 `team`

结果：

- 只给最近这个 `direct` 结算直推返佣
- 不再向上结算团队返佣

说明：

- 这条规则用于阻止“付款用户本身已经具备返佣身份时，再继续向上形成团队极差”

#### Mode 3: `direct_with_team_chain`

条件：

- 付款用户的直接邀请人身份是 `direct`
- 且付款用户身份是 `normal`

结果：

- 最近这个 `direct` 结算直推返佣
- 再从该 `direct` 的上级开始向上遍历
- 收集链路中所有身份为 `team` 的祖先节点
- 忽略链路中的 `normal` 和 `direct`
- 命中的团队节点按距离递减分团队差额池

### Team Chain Collection

只在 `direct_with_team_chain` 模式下执行。

步骤：

1. 从最近直推邀请人的直接上级开始向上遍历整条祖先链
2. 遇到 `team` 节点则收集
3. 遇到 `normal` 或 `direct` 节点则跳过，但继续向上
4. 命中的团队节点按距离从近到远排序：
   - `T1`
   - `T2`
   - `T3`
5. 超过 `team_max_depth` 的团队节点不参与分配

这意味着以下链路都合法：

- `team1 -> direct1 -> normalUser`
- `team2 -> team1 -> direct1 -> normalUser`
- `team2 -> direct2 -> team1 -> direct1 -> normalUser`

其中：

- 只有最近的 `direct1` 能拿直推返佣
- 所有命中的 `team` 节点才参与团队差额池
- 链路里其他 `direct` 节点都不拿返佣

## Configuration Model

每个订阅分组 `upgrade_group` 维护一套独立的两级订阅返佣方案。

### Scheme Fields

建议字段：

- `group`
- `enabled`
- `direct_cap_bps`
- `team_cap_bps`
- `team_decay_ratio`
- `team_max_depth`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

### Hard Constraints

- `0 <= direct_cap_bps <= team_cap_bps <= 10000`
- `team_decay_ratio` 必须在 `(0, 1]`
- `team_max_depth >= 1`

### Meaning

- `direct_cap_bps`
  - 直推返佣上限
- `team_cap_bps`
  - 团队累计返佣上限
- `team_cap_bps - direct_cap_bps`
  - 团队差额池

当订单进入：

- `team_direct` 模式时
  - 直接给团队本人结算到 `team_cap_bps`
- `direct_only` 模式时
  - 只结算 `direct_cap_bps`
- `direct_with_team_chain` 模式时
  - 最近直推拿 `direct_cap_bps`
  - 命中的团队节点分 `team_cap_bps - direct_cap_bps`

## Settlement Formula

### Base

统一口径：

- `B = paid_amount × QuotaPerUnit`

所有返佣最终仍结算为系统内 quota。

### Reward Types

新模型只有三种返佣明细类型：

- `direct_reward`
- `team_reward`
- `team_direct_reward`

#### 1. Direct Reward

适用于：

- `direct_only`
- `direct_with_team_chain`

公式：

- `direct_reward = floor(B × direct_cap_bps / 10000)`

归属：

- 始终只给最近直接邀请付款用户的那个 `direct`

#### 2. Team Pool

只在 `direct_with_team_chain` 模式存在。

公式：

- `team_pool = floor(B × (team_cap_bps - direct_cap_bps) / 10000)`

若：

- `team_pool <= 0`
- 或未命中任何 `team` 节点

则：

- 不生成任何团队返佣明细

#### 3. Team Direct Reward

只在 `team_direct` 模式存在。

公式：

- `team_direct_reward = floor(B × team_cap_bps / 10000)`

归属：

- 始终只给最近直接邀请付款用户的那个 `team`

### Team Pool Allocation

对命中的团队节点 `T1, T2, T3 ... Tk`，设：

- `r = team_decay_ratio`

则权重为：

- `w1 = 1`
- `w2 = r`
- `w3 = r^2`
- `...`

归一化后：

- `share_i = w_i / Σw`
- `reward_i = floor(team_pool × share_i)`

尾差处理建议：

- 所有向下取整后的尾差统一补给最近团队节点 `T1`

原因：

- 对账最稳定
- 用户最容易理解

## Worked Examples

假设：

- `B = 1000 quota`
- `direct_cap_bps = 1000`
- `team_cap_bps = 2500`
- `team_decay_ratio = 0.5`
- `team_max_depth = 10`

则：

### Example 1: `direct1 -> normalUser`

模式：

- `direct_with_team_chain`

结果：

- `direct1 = 100`
- 无团队返佣

### Example 2: `direct2 -> direct1 -> normalUser`

模式：

- `direct_with_team_chain`

结果：

- `direct1 = 100`
- `direct2 = 0`

说明：

- 上层 `direct2` 不参与返佣

### Example 3: `team1 -> direct1 -> normalUser`

模式：

- `direct_with_team_chain`

结果：

- `direct1 = 100`
- `team1 = 150`

### Example 4: `team2 -> team1 -> direct1 -> normalUser`

模式：

- `direct_with_team_chain`

结果：

- `direct1 = 100`
- 团队池 = `150`
- 团队权重：
  - `team1 = 1`
  - `team2 = 0.5`
- 团队分配结果：
  - `team1 = 100`
  - `team2 = 50`

### Example 5: `team2 -> direct2 -> team1 -> direct1 -> normalUser`

模式：

- `direct_with_team_chain`

结果：

- `direct1 = 100`
- 团队池 = `150`
- 命中的团队节点：
  - `team1`
  - `team2`
- `direct2` 不参与返佣

### Example 6: `team1 -> normalUser`

模式：

- `team_direct`

结果：

- `team1 = 250`

### Example 7: `direct1 -> team1 -> normalUser`

模式：

- `team_direct`

结果：

- `team1 = 250`
- `direct1 = 0`

说明：

- 因为付款用户的直接邀请人是 `team1`
- 所以不触发返佣链

### Example 8: `team1 -> direct1 -> directUser`

模式：

- `direct_only`

结果：

- `direct1 = 100`
- `team1 = 0`

### Example 9: `team1 -> direct1 -> teamUser`

模式：

- `direct_only`

结果：

- `direct1 = 100`
- `team1 = 0`

### Example 10: `normalA -> direct1`

条件：

- `direct1` 作为付款用户购买订阅

结果：

- `normalA = 0`

说明：

- `normal` 用户不能拿订阅返佣

### Example 11: `team1 -> normalA -> normalUser`

结果：

- 整笔不返佣

说明：

- 付款用户的直接邀请人 `normalA` 不是返佣身份
- 不允许跳过 `normalA` 继续给 `team1`

## Data Model

### 1. User Identity

建议在 `users` 上新增：

- `referral_level_type`

枚举：

- `normal`
- `direct`
- `team`

说明：

- `inviter_id` 仍然是邀请关系的业务真相
- `referral_level_type` 是新的返佣身份真相

### 2. Scheme Table

新增表：`subscription_level_referral_schemes`

建议字段：

- `id`
- `group`
- `enabled`
- `direct_cap_bps`
- `team_cap_bps`
- `team_decay_ratio`
- `team_max_depth`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

### 3. Settlement Batch

新增表：`subscription_level_referral_batches`

一笔订单最多对应一条返佣批次。

建议字段：

- `id`
- `order_id`
- `trade_no`
- `payer_user_id`
- `payer_level_type_snapshot`
- `direct_inviter_user_id`
- `direct_inviter_level_type_snapshot`
- `group`
- `plan_id`
- `paid_amount`
- `quota_per_unit_snapshot`
- `scheme_snapshot_json`
- `team_chain_snapshot_json`
- `settlement_mode`
  - `team_direct`
  - `direct_only`
  - `direct_with_team_chain`
- `status`
- `settled_at`
- `created_at`
- `updated_at`

### 4. Settlement Records

新增表：`subscription_level_referral_records`

一笔订单可以对应多条返佣明细。

建议字段：

- `id`
- `batch_id`
- `trade_no`
- `beneficiary_user_id`
- `beneficiary_level_type`
- `reward_type`
  - `direct_reward`
  - `team_reward`
  - `team_direct_reward`
- `depth`
- `weight_snapshot`
- `share_snapshot`
- `applied_rate_bps`
- `reward_quota`
- `reversed_quota`
- `debt_quota`
- `status`
- `created_at`
- `updated_at`

说明：

- `depth`
  - 对于团队节点，表示在命中的团队链里的距离深度
- `weight_snapshot`
  - 保存递减计算时的原始权重
- `share_snapshot`
  - 保存归一化后的实际分配比例

## Admin Surfaces

本次至少需要两个后台能力：

### 1. 返佣方案配置

按 `upgrade_group` 维护：

- `enabled`
- `direct_cap_bps`
- `team_cap_bps`
- `team_decay_ratio`
- `team_max_depth`

### 2. 用户身份管理

后台为用户指定：

- `normal`
- `direct`
- `team`

同时继续维护用户直接邀请人。

保存时至少校验：

- 不能自己邀请自己
- 不能形成环

## Rollout and Compatibility

### Engine Boundary

新模型与当前旧版订阅返佣引擎并存，但语义独立。

建议策略：

- 若某个 `upgrade_group` 命中新模型方案，且 `enabled = true`
  - 只执行新模型结算
  - 跳过旧版 `subscription_referral_records` 逻辑
- 若该分组未命中新模型方案
  - 继续保持旧逻辑

这样可以：

- 按分组灰度迁移
- 避免同一笔订单被两套返佣引擎重复结算

### Existing UI Boundary

当前用户侧“邀请返佣”页面服务的是旧版“邀请人 / 被邀请人分账”模型。

本次方案默认：

- 不复用旧页面语义
- 不要求邀请人继续手动配置“分给被邀请人的比例”
- 新模型优先以后台配置和结算引擎落地

## Acceptance Criteria

满足以下条件即可认为本次方案成立：

1. 管理员可以按 `upgrade_group` 配置两级返佣方案
2. 管理员可以给用户指定 `normal / direct / team` 身份
3. 付款用户直接邀请人为 `normal` 时，整笔不返佣
4. 付款用户直接邀请人为 `team` 时，只给该 `team` 本人返佣
5. 付款用户直接邀请人为 `direct` 且付款用户为 `normal` 时：
   - 最近 `direct` 拿直推返佣
   - 命中的 `team` 节点拿团队差额返佣
6. 付款用户直接邀请人为 `direct` 且付款用户为 `direct / team` 时：
   - 只给最近 `direct` 返佣
   - 不向上结算团队返佣
7. 历史订单在修改身份、邀请关系、返佣方案后仍保持原结算结果不变
8. 同一笔订单不会同时命中新旧两套返佣引擎
