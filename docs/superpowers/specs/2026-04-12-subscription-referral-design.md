# Subscription Referral Design

## Goal

为 `hermestoken` 的订阅套餐增加一套参考 OKX 分润模型的“邀请人 / 被邀请人”返佣机制。

本次目标是：

- 只有订阅真实支付成功才触发返佣
- 管理员可以配置全局总返佣率
- 管理员可以按邀请人单独覆盖总返佣率
- 邀请人可以在现有“邀请奖励”页面里配置分给被邀请人的比例
- 邀请人和被邀请人的返佣都进入现有 `aff_quota / aff_history_quota` 奖励池
- 订阅退款时可回退未划转奖励

## Context

当前项目已有两套可复用的基础能力：

1. 现有邀请体系
   - `user.inviter_id`
   - `user.aff_code`
   - `user.aff_quota`
   - `user.aff_history_quota`
   - 前端已有“邀请奖励”展示与“划转到余额”入口

2. 新订阅体系
   - `subscription_plans`
   - `subscription_orders`
   - `user_subscriptions`
   - `CompleteSubscriptionOrder(tradeNo, providerPayload)` 作为统一订单完成入口

本次不是重做旧邀请体系，而是在其旁边新增一套“订阅返佣规则”，并复用现有邀请奖励池与前端入口。

## Confirmed Product Decisions

以下规则已确认：

- 只对“真实支付成功”的订阅订单结算返佣
- 管理员手动绑定 / 赠送订阅不结算返佣
- 同一个被邀请人后续每一笔成功订阅订单都持续返佣
- 返佣基数按订单“实际支付成功金额”计算，不按套餐标价计算
- 管理员配置的是“总返佣率”
- 邀请人自己配置的是“分给被邀请人的比例”
- 邀请人和被邀请人的比例共用同一返佣池
- 被邀请人的奖励不是下单立减，而是支付成功后进入自己的奖励池
- 默认“分给被邀请人的比例”为 `0%`
- 退款时优先回退尚未划转的奖励，已划转不足部分不强扣，只记录欠账
- 返佣最终进入现有 `aff_quota / aff_history_quota`
- 金额换算口径使用现有系统额度逻辑：`money * QuotaPerUnit`

## Out of Scope

以下内容不在本次实现范围：

- 被邀请人维度的单独差异化比例配置
- 下单时直接折扣
- 推广报表、排行榜、大盘分析页
- 对现有老邀请体系做全面抽象重构
- 对“划转邀请奖励”做逐笔归因分摊

## Approaches Considered

### Approach A: 直接复用 `aff_quota`，不建返佣流水

做法：

- 订单成功后直接把返佣加到 `aff_quota / aff_history_quota`
- 不记录独立订阅返佣流水

优点：

- 改动最小
- 接入快

缺点：

- 无法可靠审计
- 退款回退困难
- 难以排查重复结算或比例来源

### Approach B: 返佣账本独立，奖励池复用现有 `aff_quota`

做法：

- 新增全局配置、邀请人覆盖配置、返佣流水
- 订单成功后先生成返佣记录，再把结果入到 `aff_quota`
- 退款时依据返佣记录回退

优点：

- 规则清晰
- 支持审计和退款
- 与现有页面兼容
- 历史订单不受后续配置变更影响

缺点：

- 比最小方案多一层模型和接口

### Approach C: 重构为统一邀请引擎

做法：

- 把注册奖励和订阅返佣都抽象进一套新的通用 referral engine

优点：

- 结构统一

缺点：

- 明显超出当前范围
- 风险和改动面过大

## Decision

采用 **Approach B**。

原因：

- 它满足本次 OKX 式分润需求的关键点：总返佣池、邀请人让利、历史快照、退款回退
- 它可以继续复用现有 `aff_quota` 页面和“划转到余额”能力
- 它避免把整个老邀请体系重做

## Business Model

### Core Roles

- 平台：收取订阅订单的真实支付金额
- 邀请人：从总返佣池中获得净返佣
- 被邀请人：从邀请人让利的部分中获得返佣

### Core Formula

统一口径：

- `订单可结算金额 = 订单实付金额`
- `总返佣额度 = 订单可结算金额 × QuotaPerUnit × 管理员总返佣率`
- `被邀请人返佣额度 = 订单可结算金额 × QuotaPerUnit × 邀请人设置的被邀请人比例`
- `邀请人净返佣额度 = 总返佣额度 - 被邀请人返佣额度`

### Ratio Storage

比例统一按 `Bps` 持久化：

- `10000 = 100%`
- `2000 = 20%`
- `350 = 3.5%`

不使用浮点数作为数据库主口径。

### Guard Rules

- `被邀请人比例 >= 0`
- `被邀请人比例 <= 当前生效总返佣率`
- 若邀请人配置超限，则自动裁剪到总返佣率
- 没有邀请人时，订阅主流程成功但不结算返佣
- 邀请人不存在、邀请关系异常、自邀请等情况不阻断订阅成功，只跳过返佣并记录日志

## Configuration Model

### Global Options

继续复用 `Option` 体系，新增：

- `SubscriptionReferralEnabled`
- `SubscriptionReferralGlobalRateBps`

用途：

- 控制订阅返佣总开关
- 控制默认总返佣率

### Per-Inviter Override

新增表：`subscription_referral_overrides`

建议字段：

- `id`
- `user_id`
- `total_rate_bps`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

说明：

- `user_id` 指向邀请人
- 后台虽然支持通过“用户名或 id”指定设置，但最终只存 `user_id`

### Inviter Self Configuration

扩展 `dto.UserSetting`，新增：

- `subscription_referral_invitee_rate_bps`

含义：

- 邀请人给所有被邀请人的统一分润比例
- 默认值 `0`
- 仅影响后续新订单，不回溯历史订单

### Effective Priority

当前邀请人的生效总返佣率：

1. 邀请人专属覆盖 `total_rate_bps`
2. 全局默认 `SubscriptionReferralGlobalRateBps`

当前被邀请人生效比例：

- `min(inviter_setting_rate_bps, effective_total_rate_bps)`

## Referral Ledger Model

新增表：`subscription_referral_records`

建议字段：

- `id`
- `order_id`
- `order_trade_no`
- `plan_id`
- `payer_user_id`
- `inviter_user_id`
- `beneficiary_user_id`
- `beneficiary_role`
- `order_paid_amount`
- `quota_per_unit_snapshot`
- `total_rate_bps_snapshot`
- `invitee_rate_bps_snapshot`
- `applied_rate_bps`
- `reward_quota`
- `reversed_quota`
- `debt_quota`
- `status`
- `created_at`
- `updated_at`

说明：

- 每笔成功订阅订单最多生成两条返佣流水：
  - 一条给邀请人
  - 一条给被邀请人
- 历史流水保存比例和 `QuotaPerUnit` 快照
- 退款依据该表回退

### Recommended Constraints

- 唯一约束：`order_id + beneficiary_user_id + beneficiary_role`

目的：

- 防止重复 webhook 导致重复返佣

## API Design

### User APIs

建议新增：

- `GET /api/user/referral/subscription`
- `PUT /api/user/referral/subscription`

`GET` 返回：

- `enabled`
- `total_rate_bps`
- `invitee_rate_bps`
- `inviter_rate_bps`
- `pending_reward_quota`
- `history_reward_quota`
- `inviter_count`

`PUT` 请求体：

- `invitee_rate_bps`

行为：

- 更新邀请人的“分给被邀请人的比例”
- 自动校验不能大于当前总返佣率

### Admin APIs

建议新增：

- `GET /api/subscription/admin/referral/settings`
- `PUT /api/subscription/admin/referral/settings`
- `GET /api/subscription/admin/referral/users/:id`
- `PUT /api/subscription/admin/referral/users/:id`
- `DELETE /api/subscription/admin/referral/users/:id`

用途：

- 读写全局开关与全局总返佣率
- 读写某个邀请人的总返佣率覆盖
- 清空覆盖，恢复走全局默认

若需要用户名定位，可复用现有用户搜索，不新增单独“用户名设置”数据结构。

## UI Design

### Inviter Side

入口继续放在现有“邀请奖励”卡片区域。

新增一个配置卡：

- 标题：`订阅返佣分配`
- 只读：`我的总返佣率`
- 可编辑：`分给被邀请人的比例`
- 只读：`我实际获得的比例`
- 按钮：`保存`

说明：

- 邀请人只能改“分给被邀请人的比例”
- 总返佣率只展示，不可编辑

同时更新现有邀请说明文案，明确订阅返佣场景：

- 被邀请人订阅支付成功后，邀请人和被邀请人可按规则获得奖励
- 奖励先进入邀请奖励池，再通过现有划转入口转入余额

### Admin Side

管理端新增两处：

1. 系统设置中的“订阅返佣”配置区
   - 开关
   - 全局总返佣率

2. 用户编辑弹窗中的“订阅返佣覆盖”
   - 该邀请人的总返佣率覆盖值
   - 清空覆盖按钮

## Core Execution Flow

### Successful Payment

支付成功后统一进入：

- `CompleteSubscriptionOrder(tradeNo, providerPayload)`

建议事务顺序：

1. 锁定订阅订单
2. 若订单已成功，直接返回
3. 校验订单状态必须为 `pending`
4. 读取订阅套餐
5. 创建 `UserSubscription`
6. 更新订单状态为成功
7. 结算订阅返佣
8. 提交事务

### Referral Settlement

返佣结算逻辑：

1. 检查返佣开关是否开启
2. 读取被邀请人的 `inviter_id`
3. 若无邀请人则跳过
4. 计算邀请人的生效总返佣率
5. 读取邀请人设置的被邀请人比例
6. 做上限裁剪
7. 使用订单 `money` 和当前 `QuotaPerUnit` 计算返佣额度
8. 分别为邀请人 / 被邀请人创建返佣流水
9. 分别增加两人的 `aff_quota / aff_history_quota`
10. 写业务日志

### Why Snapshot Values

以下配置都可能在未来变化：

- 全局总返佣率
- 邀请人覆盖总返佣率
- 邀请人分给被邀请人的比例
- `QuotaPerUnit`

历史订单必须保持原口径，所以返佣流水必须保存快照值。

## Refund / Reversal Flow

建议提供统一能力：

- `ReverseSubscriptionReferralByOrder(tradeNo string)`

处理规则：

1. 找到该订单下所有返佣流水
2. 逐条计算可回退额度
3. 优先扣减用户当前 `aff_quota`
4. 若 `aff_quota` 不足，剩余部分记入 `debt_quota`
5. 更新流水状态为：
   - `reversed`
   - `partially_reversed`

注意：

- 不从主 `quota` 或余额强扣
- 不阻断主退款流程

## Error Handling

以下情况不应阻断订阅成功：

- 被邀请人没有邀请人
- 邀请人不存在
- 邀请关系异常
- 当前返佣率为 `0`

这些情况只跳过返佣并记录日志。

以下情况需要强约束：

- `invitee_rate_bps < 0`
- `invitee_rate_bps > effective_total_rate_bps`
- 管理员设置非法全局返佣率
- 管理员设置非法邀请人覆盖返佣率

## Testing Scope

### Backend Tests

至少覆盖：

- 没有邀请人时，订阅成功但不返佣
- 有邀请人时，按全局返佣率正确结算
- 有用户覆盖返佣率时，覆盖优先生效
- 被邀请人比例为 `0` 时，全额归邀请人
- 被邀请人比例小于总返佣率时，双方按比例分账
- 被邀请人比例大于总返佣率时，被自动裁剪
- 同一被邀请人多次订阅，每次都返佣
- 重复 webhook 不重复发奖励
- 退款时正确回退未划转额度
- `aff_quota` 不足以回退时，正确记录 `debt_quota`

### Frontend Tests

至少覆盖：

- 邀请奖励页能展示总返佣率 / 被邀请人比例 / 邀请人实际比例
- 保存被邀请人比例时校验通过
- 超过总返佣率时前端拦截
- 管理员修改全局配置后，页面展示正确刷新
- 管理员设置邀请人覆盖后，邀请人页展示正确生效值

## Delivery Boundary

### Must Deliver

- 全局开关与全局总返佣率
- 按邀请人覆盖总返佣率
- 邀请人自定义被邀请人比例
- 订阅订单成功自动结算返佣
- 奖励进入现有邀请奖励池
- 退款回退未划转奖励
- 返佣流水可追踪

### Not Delivering Now

- 下单立减
- 被邀请人级别单独配置
- 推广报表与榜单
- 对旧邀请体系做系统级重构

## Implementation Notes

建议优先把返佣结算逻辑挂在 `CompleteSubscriptionOrder` 内部或其事务辅助函数中，不在各支付控制器里各写一套。

这样：

- `epay`
- `stripe`
- `creem`

三条支付链路都会统一复用一套返佣结算与幂等逻辑。
