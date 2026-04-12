# Subscription Referral By Group Design

## Goal

为 `hermestoken` 的订阅返佣机制增加“按订阅分组生效”的作用域。

本次目标是：

- 管理员按分组配置订阅返佣总返佣率，而不是只配置一个全局值
- 邀请人的订阅返佣覆盖也按分组配置
- 邀请人分给被邀请人的比例也按分组配置
- 只有购买了命中某个订阅分组的套餐时，才按该分组的规则触发返佣
- 没有 `upgrade_group` 的订阅套餐默认不参与订阅返佣
- 不同分组之间的返佣规则、覆盖和用户分配彼此独立
- 历史返佣记录保存命中的分组和费率快照，避免后续配置变更影响历史订单

## Context

当前项目已经具备以下基础：

1. 订阅体系
   - `subscription_plans.upgrade_group`
   - `subscription_orders`
   - `CompleteSubscriptionOrder(tradeNo, providerPayload)` 作为统一完成入口
   - 订阅成功时会基于套餐的 `upgrade_group` 更新用户分组

2. 现有订阅返佣体系
   - `SubscriptionReferralEnabled`
   - `SubscriptionReferralGlobalRateBps`
   - `subscription_referral_overrides`
   - `subscription_referral_records`
   - 邀请人自己的 `subscription_referral_invitee_rate_bps`

3. 可复用的分组能力
   - 用户主分组 `user.group`
   - 订阅套餐升级分组 `subscription_plans.upgrade_group`
   - 管理端已有分组配置页面与分组列表来源

本次不是重做整个返佣系统，而是在现有订阅返佣能力上增加“按 `upgrade_group` 作用域隔离”的规则。

## Confirmed Product Decisions

以下规则已经确认：

- 返佣生效分组以订阅套餐的 `upgrade_group` 为准
- 只有购买了命中该 `upgrade_group` 的订阅，才会按这个分组的返佣规则结算
- `upgrade_group` 为空的订阅套餐不参与订阅返佣
- 管理员全局返佣率按分组配置：`group -> total_rate_bps`
- 管理员给邀请人的返佣覆盖也按分组配置：`(user_id, group) -> total_rate_bps`
- 邀请人分给被邀请人的比例也按分组配置：`group -> invitee_rate_bps`
- 不同分组之间彼此独立，A 组的返佣规则不会影响 B 组
- 如果某个分组没有返佣配置或返佣率为 `0`，该分组订阅不返佣
- 返佣快照需要记录命中的分组与费率，保证后续回退和审计稳定

## Out of Scope

以下内容不在本次范围：

- 重新设计注册邀请奖励体系
- 为被邀请人再增加独立的分组级差异化规则
- 订阅下单时直接折扣或立减
- 新的返佣报表、排行榜、分析大盘
- 对所有旧返佣接口做一次性破坏式重写
- 对现有分组体系做无关的结构性重构

## Approaches Considered

### Approach A: 以订阅套餐 `upgrade_group` 作为返佣作用域（推荐）

做法：

- 后台分组返佣配置以 `upgrade_group` 作为键
- 用户覆盖和邀请人自定义比例也都以该分组作为键
- 订单成功时只按当前订阅套餐命中的 `upgrade_group` 结算

优点：

- 和现有订阅模型完全一致
- 规则清晰，后台和前台语义统一
- 不需要引入新的返佣维度

缺点：

- 需要同时修改后端模型、管理端 UI、用户前台 UI 和测试

### Approach B: 只让后台配置按分组，邀请人自定义比例仍保留单值

做法：

- 总返佣率和后台覆盖按分组
- 邀请人分给被邀请人的比例仍只有一份全局值

优点：

- 改动略小

缺点：

- 会出现分组返佣率差异很大时，前台显示和实际结算不直观
- 需要在结算阶段做额外裁剪，规则不自然

### Approach C: 仅增加“哪些分组参与返佣”的白名单

做法：

- 只有命中白名单分组的订阅才返佣
- 返佣率和邀请人分配仍使用现有单值

优点：

- 实现最快

缺点：

- 不满足“全局返佣按分组配置”和“用户返佣也按分组配置”的需求

## Decision

采用 **Approach A**。

原因：

- 它与用户确认的业务规则完全一致
- 它直接复用现有订阅套餐里的 `upgrade_group`，不额外制造新的返佣概念
- 它能让管理端、邀请人前台、结算逻辑和返佣记录保持同一个分组语义

## Business Rules

### Trigger Scope

订阅返佣的生效条件：

1. `SubscriptionReferralEnabled == true`
2. 订单真实支付成功并进入 `CompleteSubscriptionOrder`
3. 订单对应套餐 `upgrade_group != ""`
4. 邀请关系有效，且不是自邀请
5. 当前命中分组的生效总返佣率 `> 0`

若任一条件不成立：

- 不阻断订阅成功主流程
- 直接跳过返佣结算
- 不生成返佣记录

### Effective Priority

对于某个分组 `G`：

- 生效总返佣率：
  1. 邀请人分组覆盖 `(user_id, G)`
  2. 全局分组返佣率 `group_rates[G]`
  3. 若新分组配置整体仍为空，则临时回退旧 `SubscriptionReferralGlobalRateBps`
  4. 一旦管理员保存过新的分组配置，缺失分组即视为 `0`

- 生效被邀请人返佣率：
  1. 邀请人设置中的 `subscription_referral_invitee_rate_bps_by_group[G]`
  2. 若没有该分组值，则兼容回退旧单值 `subscription_referral_invitee_rate_bps`
  3. 最终值裁剪为 `min(invitee_rate_bps, effective_total_rate_bps)`

### Formula

统一口径：

- `订单可结算金额 = 订单实付金额`
- `总返佣额度 = 订单可结算金额 × QuotaPerUnit × 当前分组总返佣率`
- `被邀请人返佣额度 = 订单可结算金额 × QuotaPerUnit × 当前分组被邀请人比例`
- `邀请人净返佣额度 = 总返佣额度 - 被邀请人返佣额度`

比例仍统一使用 `Bps` 存储：

- `10000 = 100%`
- `4500 = 45%`
- `125 = 1.25%`

## Data Model

### Global Option Model

保留：

- `SubscriptionReferralEnabled`

新增：

- `SubscriptionReferralGroupRates`

建议结构：

```json
{
  "default": 4500,
  "vip": 3000,
  "pro": 1500
}
```

说明：

- key 为订阅套餐 `upgrade_group`
- value 为该分组总返佣率（BPS）
- 后台页面中“禁用某个分组返佣”最终落库为该分组费率 `0`
- 旧 `SubscriptionReferralGlobalRateBps` 保留为兼容迁移输入，不再作为长期主配置

### Per-Inviter Override Model

扩展 `subscription_referral_overrides`：

- 现状：`user_id` 唯一
- 目标：`(user_id, group)` 唯一

建议字段：

- `id`
- `user_id`
- `group`
- `total_rate_bps`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

说明：

- `user_id` 指向邀请人
- `group` 指向订阅套餐命中的 `upgrade_group`
- 同一邀请人可同时拥有多个分组覆盖

### Inviter Self Configuration Model

扩展 `dto.UserSetting`：

新增：

- `subscription_referral_invitee_rate_bps_by_group`

建议结构：

```json
{
  "default": 1500,
  "vip": 500
}
```

兼容保留：

- `subscription_referral_invitee_rate_bps`

说明：

- 新字段作为长期主配置
- 老字段只作为兼容回退读取使用
- 只有当前订阅命中的分组会参与读取

### Referral Ledger Model

扩展 `subscription_referral_records`：

新增字段：

- `referral_group`

每条返佣记录需要保存：

- 命中的 `referral_group`
- `total_rate_bps_snapshot`
- `invitee_rate_bps_snapshot`
- `applied_rate_bps`
- `quota_per_unit_snapshot`

说明：

- 返佣快照必须足以独立解释这笔返佣如何计算出来
- 后续管理员修改分组返佣率、用户覆盖或邀请人分配比例时，不影响历史记录

## Backend Design

### In-Memory Option Handling

新增一套与现有 `TopupGroupRatio` 类似的内存配置：

- `SubscriptionReferralGroupRates2JSONString()`
- `UpdateSubscriptionReferralGroupRatesByJSONString(jsonStr string)`
- `GetSubscriptionReferralGroupRate(group string) int`
- `HasSubscriptionReferralGroupRatesConfigured() bool`

同时更新 `model/option.go`：

- `InitOptionMap` 将 `SubscriptionReferralGroupRates` 放入 `OptionMap`
- `UpdateOption` 在更新该 key 时同步刷新内存态
- 允许老配置和新配置共存，但结算优先走新分组配置逻辑

### Effective Resolution Helpers

新增或重构以下 helper：

- `GetSubscriptionReferralOverrideByUserIDAndGroup(userID int, group string)`
- `UpsertSubscriptionReferralOverride(userID int, group string, totalRateBps int, operatorID int)`
- `DeleteSubscriptionReferralOverrideByUserIDAndGroup(userID int, group string)`
- `GetEffectiveSubscriptionReferralTotalRateBps(userID int, group string)`
- `GetEffectiveSubscriptionReferralInviteeRateBps(setting dto.UserSetting, group string, totalRateBps int)`
- `ResolveSubscriptionReferralConfig(totalRateBps int, inviteeRateBps int)`

说明：

- `ResolveSubscriptionReferralConfig` 不再自己读取 `UserSetting`，而是接收已经解析好的分组级 `inviteeRateBps`
- 这样分组逻辑和配置读取逻辑分层更清楚

### Settlement Flow

`ApplySubscriptionReferralOnOrderSuccessTx` 需要改成按订单命中的订阅分组结算。

推荐流程：

1. 从 `SubscriptionPlan` 读取 `upgrade_group`
2. `upgrade_group == ""` 直接返回
3. 查被邀请人、邀请人
4. 解析当前分组生效总返佣率
5. 若总返佣率 `<= 0`，直接返回
6. 解析邀请人给被邀请人的当前分组比例，并裁剪到总返佣率以内
7. 生成 inviter / invitee 两条返佣记录，并写入 `referral_group`
8. 更新双方 `aff_quota / aff_history_quota`

关键行为：

- 分组不存在、分组费率无效、邀请关系异常时都不阻断主支付流程
- 返佣作用域只由本次订阅套餐的 `upgrade_group` 决定，不读取邀请人或被邀请人当前 `user.group` 作为主判断依据

## API Design

### Admin Settings API

保留路由：

- `GET /api/subscription/admin/referral/settings`
- `PUT /api/subscription/admin/referral/settings`

返回结构改为：

```json
{
  "enabled": true,
  "group_rates": {
    "default": 4500,
    "vip": 3000
  }
}
```

保存结构改为：

```json
{
  "enabled": true,
  "group_rates": {
    "default": 4500,
    "vip": 3000
  }
}
```

要求：

- 只允许保存当前系统已有的分组
- 对每个分组费率做 `0..10000` 的归一化

### Admin Inviter Override API

保留路由前缀：

- `GET /api/subscription/admin/referral/users/:id`
- `PUT /api/subscription/admin/referral/users/:id`
- `DELETE /api/subscription/admin/referral/users/:id`

行为调整：

- `GET` 一次性返回该用户所有分组的返佣视图
- `PUT` 的 body 带 `group` 和 `total_rate_bps`
- `DELETE` 通过 query 或 body 指定 `group`

推荐 `GET` 返回：

```json
{
  "user_id": 123,
  "groups": [
    {
      "group": "default",
      "effective_total_rate_bps": 4500,
      "has_override": false,
      "override_rate_bps": null
    },
    {
      "group": "vip",
      "effective_total_rate_bps": 3000,
      "has_override": true,
      "override_rate_bps": 2500
    }
  ]
}
```

### User Self Referral API

保留路由：

- `GET /api/user/referral/subscription`
- `PUT /api/user/referral/subscription`

`GET` 返回自己的分组返佣配置视图：

```json
{
  "enabled": true,
  "groups": [
    {
      "group": "default",
      "total_rate_bps": 4500,
      "invitee_rate_bps": 500,
      "inviter_rate_bps": 4000
    },
    {
      "group": "vip",
      "total_rate_bps": 3000,
      "invitee_rate_bps": 0,
      "inviter_rate_bps": 3000
    }
  ],
  "pending_reward_quota": 0,
  "history_reward_quota": 0,
  "inviter_count": 0
}
```

`PUT` 请求结构：

```json
{
  "group": "vip",
  "invitee_rate_bps": 500
}
```

要求：

- `group` 必须存在于当前返佣可配置分组中
- `invitee_rate_bps` 不能超过该分组当前生效总返佣率

## Frontend Design

### Admin Settings Page

页面：`web/src/pages/Setting/Operation/SettingsSubscriptionReferral.jsx`

改动：

- 保留顶层开关 `启用订阅返佣`
- 将单个 `全局总返佣率` 输入框改为分组列表/表格
- 表格列建议：
  - `分组`
  - `总返佣率`
  - `是否启用返佣`
- 保存时最终归一成 `group_rates`
- 分组来源使用系统已有分组列表，不创建新的返佣分组体系

页面文案需明确：

- “只有购买升级到该分组的订阅时，才按该分组返佣”

### Admin User Override Section

页面：`web/src/components/table/users/modals/SubscriptionReferralOverrideSection.jsx`

改动：

- 从单个覆盖卡片改为按分组展示的覆盖列表
- 每个分组展示：
  - 当前生效总返佣率
  - 当前是否有覆盖
  - 覆盖总返佣率输入框
  - 清除该分组覆盖按钮
- 建议一次加载全部分组，避免逐个分组请求

### User Invitation Page

页面：`web/src/components/topup/InvitationCard.jsx` 和相关数据加载逻辑

改动：

- 从一套总返佣率配置改成按分组卡片或折叠面板
- 每个分组展示：
  - 我的总返佣率
  - 分给被邀请人的比例
  - 我实际获得的比例
  - 输入框与保存按钮
- 只展示当前参与订阅返佣的有效分组

页面文案需明确：

- “不同订阅分组的返佣比例彼此独立”
- “只有购买该分组订阅时，本分组返佣规则才会生效”

## Compatibility and Migration

### Global Group Rates Migration

为避免升级后老站点的订阅返佣突然全部失效：

- 若 `SubscriptionReferralGroupRates` 为空，则读取旧 `SubscriptionReferralGlobalRateBps` 作为临时回退
- 后台首次保存新的 `group_rates` 后，即视为站点已完成迁移
- 迁移完成后，缺失分组按 `0` 处理，不再回退旧单值

### Inviter Self Setting Migration

为避免老用户升级后“分给被邀请人的比例”突然全部变成 `0`：

- 优先读取 `subscription_referral_invitee_rate_bps_by_group[group]`
- 若该分组未配置，则回退到老字段 `subscription_referral_invitee_rate_bps`
- 用户在前台保存任一分组值后，新字段开始逐步接管

### Database Migration Notes

本次数据库变更需要兼容已有数据：

- 为 `subscription_referral_overrides` 增加 `group` 字段，并改唯一索引到 `(user_id, group)`
- 为 `subscription_referral_records` 增加 `referral_group` 字段
- 老的 override 数据在迁移后可视为“未区分分组的旧覆盖”，不直接自动复制到所有分组；由兼容回退逻辑和后台重新保存来完成平滑过渡

## Error Handling

### Validation Rules

- 后台保存分组费率时：
  - 分组必须存在
  - 费率必须可归一化到 `0..10000`

- 用户保存分组分成时：
  - 分组必须存在
  - 比例必须 `>= 0`
  - 比例必须 `<= 当前分组生效总返佣率`

- 删除分组覆盖时：
  - 只删除该分组的覆盖
  - 不影响同一用户的其他分组覆盖

### Failure Policy

- 返佣结算错误不应影响订阅支付成功的主结果，除非错误意味着当前事务数据已经部分写入且无法保持一致
- 对于“无需返佣”的业务分支（如没有 `upgrade_group`、分组费率为 `0`、邀请关系无效），应静默跳过，不视为异常

## Testing Strategy

### Model Tests

覆盖：

- 分组返佣率解析与归一化
- `(user_id, group)` 覆盖命中与回退
- 邀请人按分组分配比例读取与旧字段兼容回退
- `ResolveSubscriptionReferralConfig` 的边界裁剪

### Settlement Tests

覆盖：

- `upgrade_group` 为空时不返佣
- 分组返佣率为 `0` 时不返佣
- 命中分组全局返佣率时正确分账
- 命中分组覆盖时正确分账
- 历史记录写入 `referral_group` 与费率快照
- 同一订单重复完成仍保持幂等

### Controller Tests

覆盖：

- 管理端按分组保存/读取返佣设置
- 管理端按分组保存/删除用户覆盖
- 用户端按分组读取和保存自己的返佣分配
- 超过分组总返佣率时返回校验错误

### Frontend Tests

覆盖：

- 管理端分组返佣表单的序列化/反序列化
- 用户覆盖分组列表的数据转换
- 用户前台分组返佣卡片的边界展示
- 旧字段兼容回显逻辑

## Implementation Notes

为了降低改动风险，推荐实现顺序：

1. 先补后端分组配置与解析 helper
2. 再改结算逻辑与返佣记录快照
3. 然后改管理端设置页
4. 再改管理端用户覆盖页
5. 最后改用户前台返佣配置页
6. 全链路补齐模型、控制器和前端测试

这样可以让返佣结算核心先稳定，再推进 UI 与兼容回显。
