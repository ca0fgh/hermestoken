# Subscription Network Referral Design

## Goal

为 `hermestoken` 的订阅体系设计一套新的“网络返佣”产品方案，支持：

- 级别间极差返佣
- 级别池子内距离递减返佣
- `项目` 下面有 `项目`
- `团队` 下面有 `团队`
- `直推` 下面有 `直推`
- 每个级别都可以直接邀请用户

本次目标不是在现有“邀请人 / 被邀请人”二分账模型上继续打补丁，而是明确一套可独立实施的多级订阅返佣规则，并复用现有订阅套餐分组 `upgrade_group` 作为返佣方案的配置入口。

## Context

当前项目的订阅返佣实现仍是二元模型：

- 订单支付成功后只生成两条返佣记录：`inviter` 和 `invitee`
- 总返佣率主要依赖 `(user_id, group)` override
- 用户端只配置“分给被邀请人的比例”

这套模型可以支持“邀请人让利给被邀请人”，但无法支持：

- 多级同层网络
- 团队池 / 项目池
- 距离递减
- 级别间极差

因此，新方案应被视为新的“订阅网络返佣模型”，而不是对旧模型的小修小补。

## Confirmed Product Decisions

以下规则已在讨论中确认：

- 只对真实支付成功的订阅订单结算返佣
- 返佣方案按订阅套餐 `upgrade_group` 生效
- 每个用户只能有 `1` 个直接上级，可以有多个下级
- 一个用户只能被邀请绑定一次
- 每个用户只能有 `1` 种网络身份级别：
  - `direct`
  - `team`
  - `project`
- 每个级别都可以直接邀请用户
- 允许同级连续：
  - `direct -> direct`
  - `team -> team`
  - `project -> project`
- 级别间采用极差返佣
- 同一级别池子内部采用距离递减返佣
- 直接邀请人如果本身是 `team` 或 `project`，既拿直推池，也参与对应级别池
- 历史订单结算结果需要快照固化，后续改上级、改身份、改返佣参数都不影响历史订单

## Out of Scope

本次方案不包含：

- 现有旧版订阅返佣表结构的强兼容改造
- 注册奖励、首单奖励等非订阅场景返佣统一抽象
- 推广排行榜、全站经营分析大盘
- 自动智能修复错误邀请链
- 运营活动维度的额外加成系数

## Approaches Considered

### Approach A: 在旧二分账模型上叠加字段

做法：

- 继续沿用 `inviter / invitee` 两角色模型
- 额外增加 team/project 相关字段
- 结算时在旧函数里继续叠加同层和跨层逻辑

问题：

- 旧模型的核心抽象不支持多级链路
- 越往后逻辑越难审计
- 退款、冲回、对账会迅速失控

### Approach B: 每级只取最近一个节点

做法：

- 最近 `direct` 拿直推返佣
- 最近 `team` 拿团队返佣
- 最近 `project` 拿项目返佣

优点：

- 实现简单

问题：

- 无法满足“团队下面有团队、项目下面有项目也都要持续有收益”
- 无法保证上层网络经营利益

### Approach C: 级别间极差 + 级别池子内距离递减（采用）

做法：

- 先按级别累计上限做极差，切出：
  - `直推池`
  - `团队池`
  - `项目池`
- 再在每个池子内部按同级祖先链做距离递减分配

优点：

- 同时满足“极差返佣”和“多级同层持续收益”
- 能支持任意深度的 `project/team/direct` 链路
- 成本边界清晰，审计口径稳定

代价：

- 需要一套新的返佣账本和结算快照模型

## Decision

采用 **Approach C**。

原因：

- 这是唯一同时满足以下三点的方案：
  - 级别间有明确的极差边界
  - 同级多层网络可持续分佣
  - 每个级别都可以直接邀请用户
- 它能把“经营层级差异”和“网络距离差异”拆开处理，避免公式相互打架

## Business Model

### Core Concepts

定义三个返佣层：

- `直推层`
- `团队层`
- `项目层`

定义三个返佣池：

- `直推池`
- `团队池`
- `项目池`

规则分两段：

1. **级别间极差**
   - 先决定三层累计最多能返多少
2. **池子内距离递减**
   - 再决定这一层里多个同级节点如何分池子

### Network Level Semantics

每个用户节点只有一个 `level_type`：

- `direct`
- `team`
- `project`

有效返佣链路从付款用户向上追溯时，只允许按以下顺序出现：

- `direct* -> team* -> project*`

其中：

- `*` 表示 `1..n` 个连续同级节点
- 各层都允许缺失
- 允许跳级，但不允许乱序交叉

因此以下链路合法：

- `direct -> user`
- `team -> user`
- `project -> user`
- `team -> direct -> user`
- `project -> direct -> user`
- `project -> team -> user`
- `project -> team -> direct -> user`
- `project -> project -> team -> team -> direct -> direct -> user`

以下链路不作为有效返佣链处理：

- `team -> project -> user`
- `direct -> project -> team -> user`

原因：

- 一旦允许乱序交叉，极差基准会失稳
- 产品解释和后台试算也会变得不可控

## Configuration Model

每个订阅分组 `upgrade_group` 维护一套独立返佣方案。

### Scheme Fields

建议字段：

- `group`
- `enabled`
- `direct_cap_bps`
- `team_cap_bps`
- `project_cap_bps`
- `direct_decay_factor`
- `team_decay_factor`
- `project_decay_factor`
- `direct_max_depth`
- `team_max_depth`
- `project_max_depth`
- `min_reward_quota`
- `settlement_delay_hours`

### Hard Constraints

- `0 <= direct_cap_bps <= team_cap_bps <= project_cap_bps <= 10000`
- 每个 `decay_factor` 必须在 `(0, 1]`
- 每个 `max_depth >= 1`
- `min_reward_quota >= 0`

### Meaning of Caps

- `direct_cap_bps`
  - 订单打到“直推层”为止，累计最多返多少
- `team_cap_bps`
  - 订单打到“团队层”为止，累计最多返多少
- `project_cap_bps`
  - 订单打到“项目层”为止，累计最多返多少

因此级别间极差天然为：

- `直推池 = direct_cap_bps`
- `团队池 = team_cap_bps - direct_cap_bps`
- `项目池 = project_cap_bps - team_cap_bps`

但若中间层不存在，则上层对下一个实际存在层做极差。

## Settlement Formula

### Base

统一口径：

- `结算基数 B = paid_amount × QuotaPerUnit / 10000`

所有返佣最终仍换算为系统内的 quota。

### Pool Amounts

#### 1. Direct Pool

- `direct_pool = B × direct_cap_bps`

分配规则：

- 若直接邀请人链首段存在连续 `direct` 节点：
  - 由这些 `direct` 节点按距离递减分 `direct_pool`
- 若最近直接邀请人是 `team` 或 `project`：
  - `direct_pool` 100% 给该最近直接邀请人

#### 2. Team Pool

只有命中至少一个 `team` 节点时才存在：

- `team_pool = B × (team_cap_bps - direct_cap_bps)`

分配规则：

- 收集所有命中的 `team` 节点
- 从近到远按距离递减分 `team_pool`

#### 3. Project Pool

只有命中至少一个 `project` 节点时才存在。

若命中团队层：

- `project_pool = B × (project_cap_bps - team_cap_bps)`

若未命中团队层：

- `project_pool = B × (project_cap_bps - direct_cap_bps)`

分配规则：

- 收集所有命中的 `project` 节点
- 从近到远按距离递减分 `project_pool`

### Distance Decay

对某个池子，若从近到远命中节点为 `N1, N2, N3 ... Nk`，衰减系数为 `r`：

- `weight_1 = 1`
- `weight_2 = r`
- `weight_3 = r^2`
- ...
- `weight_k = r^(k-1)`

归一化后：

- `share_i = weight_i / Σweight`
- `reward_i = floor(pool_amount × share_i)`

尾差处理建议：

- 全部向下取整后，把尾差补给最近节点 `N1`

### Depth and Min Payout Guards

- 超过 `max_depth` 的同级节点不参与该池分配
- 若某节点计算出的 `reward_quota < min_reward_quota`：
  - 该节点不入账
  - 该尾部碎账继续并入最近命中节点，或统一并入平台未分配额

推荐产品口径：

- 小额尾差统一并入最近节点，避免出现“池子金额与明细金额不相等”

## Valid Chain Combinations

在上述规则下，所有有效链路可收敛为以下七类：

### 1. `direct* -> user`

- `直推池`：由 `direct*` 按距离递减分
- `团队池`：无
- `项目池`：无

### 2. `team* -> user`

- `直推池`：100% 给最近团队 `T1`
- `团队池`：由 `team*` 按距离递减分
- `项目池`：无

### 3. `project* -> user`

- `直推池`：100% 给最近项目 `P1`
- `团队池`：无
- `项目池`：按 `project_cap - direct_cap` 计算，由 `project*` 按距离递减分

### 4. `team* -> direct* -> user`

- `直推池`：由 `direct*` 按距离递减分
- `团队池`：按 `team_cap - direct_cap` 计算，由 `team*` 按距离递减分
- `项目池`：无

### 5. `project* -> direct* -> user`

- `直推池`：由 `direct*` 按距离递减分
- `团队池`：无
- `项目池`：按 `project_cap - direct_cap` 计算，由 `project*` 按距离递减分

### 6. `project* -> team* -> user`

- `直推池`：100% 给最近团队 `T1`
- `团队池`：按 `team_cap - direct_cap` 计算，由 `team*` 按距离递减分
- `项目池`：按 `project_cap - team_cap` 计算，由 `project*` 按距离递减分

### 7. `project* -> team* -> direct* -> user`

- `直推池`：由 `direct*` 按距离递减分
- `团队池`：按 `team_cap - direct_cap` 计算，由 `team*` 按距离递减分
- `项目池`：按 `project_cap - team_cap` 计算，由 `project*` 按距离递减分

## Worked Examples

### Example A

配置：

- `direct_cap = 10%`
- `team_cap = 22%`
- `project_cap = 30%`

链路：

- `P2 -> P1 -> T2 -> T1 -> D2 -> D1 -> 用户`

结果：

- `直推池 = 10%`
  - `D1、D2` 按 `direct_decay_factor` 分
- `团队池 = 12%`
  - `T1、T2` 按 `team_decay_factor` 分
- `项目池 = 8%`
  - `P1、P2` 按 `project_decay_factor` 分

### Example B

配置同上。

链路：

- `T2 -> T1 -> 用户`

结果：

- `直推池 = 10%`
  - 100% 给 `T1`
- `团队池 = 12%`
  - `T1、T2` 按团队衰减分
- `项目池 = 0`

因此 `T1` 同时拿：

- 直推池
- 团队池中的最近层份额

### Example C

配置同上。

链路：

- `P2 -> P1 -> 用户`

结果：

- `直推池 = 10%`
  - 100% 给 `P1`
- `团队池 = 0`
- `项目池 = 20%`
  - `P1、P2` 按项目衰减分

## Data Model

建议新增一套独立账本模型，不复用旧 `subscription_referral_records` 语义。

### 1. User Network Profile

建议字段：

- `users.level_type`
- `users.inviter_id`
- `users.network_path_snapshot_version`
- `users.ancestor_path`

说明：

- 业务真相仍是 `inviter_id`
- `ancestor_path` 是性能字段，不是业务真相
- 每次组织迁移后递增版本号，便于缓存失效

### 2. Subscription Network Schemes

新增表：`subscription_network_schemes`

建议字段：

- `id`
- `group`
- `enabled`
- `direct_cap_bps`
- `team_cap_bps`
- `project_cap_bps`
- `direct_decay_factor`
- `team_decay_factor`
- `project_decay_factor`
- `direct_max_depth`
- `team_max_depth`
- `project_max_depth`
- `min_reward_quota`
- `settlement_delay_hours`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

### 3. Settlement Batch

新增表：`subscription_network_batches`

一笔订单一条批次记录。

建议字段：

- `id`
- `order_id`
- `trade_no`
- `payer_user_id`
- `plan_id`
- `group`
- `paid_amount`
- `quota_per_unit_snapshot`
- `scheme_snapshot_json`
- `chain_snapshot_json`
- `status`
- `settled_at`
- `available_at`
- `created_at`
- `updated_at`

### 4. Settlement Records

新增表：`subscription_network_records`

一笔订单多条明细，每条对应一个受益节点。

建议字段：

- `id`
- `batch_id`
- `trade_no`
- `beneficiary_user_id`
- `beneficiary_level_type`
- `pool_type`
- `depth`
- `weight_snapshot`
- `share_snapshot`
- `reward_quota`
- `reversed_quota`
- `debt_quota`
- `status`
- `chain_segment_snapshot_json`
- `created_at`
- `updated_at`

`pool_type` 枚举：

- `direct`
- `team`
- `project`

## Product Surfaces

### Admin: Scheme Configuration

新增“订阅网络返佣方案”页面：

- 按 `upgrade_group` 配置返佣方案
- 支持启用 / 停用
- 支持设置 cap、decay、depth、最小入账额度、观察期

页面校验：

- cap 顺序校验
- decay 合法区间校验
- depth 至少为 `1`
- 实时展示“极差后实际池子比例”

### Admin: Network User Management

新增“网络身份与关系管理”页面：

- 查看用户当前级别
- 查看直接上级、直接下级数量
- 修改用户 `level_type`
- 迁移用户上级
- 展示组织链路预览
- 记录完整审计日志

### Admin: Commission Simulator

新增“返佣试算器”：

- 输入：
  - 付款用户
  - 订阅分组
  - 支付金额
- 输出：
  - 命中链路
  - 三个池金额
  - 每个节点应得返佣

这是后台运营和排查问题时的核心工具。

### User: My Network Rewards

用户前台展示建议按订单维度展开：

- 订单号
- 付款人
- 命中的直推链 / 团队链 / 项目链
- 自己命中的是哪个池
- 本单获得多少 quota
- 状态：
  - 待生效
  - 已生效
  - 已冲回
  - 部分冲回
  - 欠账

## Settlement Lifecycle

### Trigger

满足以下条件时才结算：

1. 订阅订单真实支付成功
2. 订单进入统一完成入口
3. 套餐 `upgrade_group != ""`
4. 对应分组方案已启用
5. 付款用户存在有效上级链

任一条件不成立：

- 不阻断订阅成功主流程
- 直接跳过返佣结算

### Settlement Flow

支付成功后：

1. 锁定订单
2. 读取付款人当前上级链
3. 解析有效返佣链段
4. 读取分组返佣方案
5. 计算三个池金额
6. 计算每个池的距离递减分配
7. 写入 `batch + records`
8. 将可立即生效的返佣入账到奖励余额
9. 若存在观察期，则先记 `pending`

### Historical Snapshot Rule

必须固定：

- 历史订单永不重算
- 支付成功时把链路和方案快照固化
- 后续改：
  - 上级
  - 级别
  - 返佣方案
  - 衰减参数
  - 最大层数
- 只影响未来订单，不影响历史订单

## Reversal and Refund Rules

### Reversal Trigger

以下场景走逆向结算：

- 全额退款
- 部分退款
- 拒付
- 风控人工撤销

### Reversal Rule

- 全额退款：所有明细按原值全部冲回
- 部分退款：所有明细按相同比例冲回
- 优先扣减当前可用返佣余额
- 不足部分记入 `debt_quota`

状态枚举建议：

- `pending`
- `available`
- `partially_reversed`
- `reversed`
- `debt`

## Migration Recommendation

推荐采用“新旧并行、逐步切换”：

1. 保留当前旧 `subscription_referral` 逻辑
2. 新增 `subscription_network_referral` 体系
3. 新分组或新活动先接入新模型
4. 旧模型不再扩展新能力

原因：

- 旧表和旧接口的语义是“邀请人 / 被邀请人二分账”
- 新方案的语义是“三层极差 + 层内递减 + 多节点分账”
- 直接强改旧模型，迁移成本和行为回归风险都过高

## Testing Strategy

必须覆盖：

- 七类合法链路组合的结算结果
- 同级连续多节点的距离递减分配
- `team/project` 直接邀请用户时同时命中直推池和本级池
- 缺失团队层时项目池对直推层做极差
- 历史订单快照不受链路变更影响
- 全额退款 / 部分退款 / 欠账
- cap、decay、depth 的后台参数校验
- 试算器结果与真实结算结果一致

## Rollout

上线顺序建议：

1. 先落库返佣方案配置与账本表结构
2. 再实现后台返佣试算器
3. 再实现真实订单结算
4. 最后开放前台返佣明细页面

上线完成标志：

- 新分组可独立配置网络返佣方案
- 后台能准确试算任意付款用户的返佣链路
- 新订单能生成 batch + records
- 退款可按快照正确冲回
- 历史快照不受组织变更影响

## Open Questions

以下问题未在本次讨论中固定，建议在实现前确认：

- 用户前台是否继续复用现有 `aff_quota / aff_history_quota`，还是拆独立余额池
- 各分组的默认衰减参数与默认 `max_depth` 是否需要提供平台级模板值
