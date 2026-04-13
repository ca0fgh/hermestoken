# Subscription Plan Stock Design

## Goal

为订阅套餐新增独立的“库存”能力，同时保留现有“购买上限”逻辑不变。

本次目标是：

- 保留当前 `购买上限` 的语义：
  - 每个用户最多可购买该套餐多少次
- 新增套餐级库存能力：
  - 套餐总库存
  - 待支付锁定库存
  - 已售库存
- 将库存校验纳入现有订阅下单与支付完成事务中
- 采用“创建支付订单时锁库存，支付成功时转已售，未支付关闭/过期时释放锁定”的规则
- 为管理端和用户端补充清晰的库存展示，避免与“购买上限”混淆

## Context

当前订阅套餐已有一个字段：

- `max_purchase_per_user`

它的现有语义是“单个用户最多可成功购买该套餐多少次”，不是套餐库存。

当前系统中的订阅订单流转大致为：

1. 创建待支付订阅订单
2. 拉起第三方支付
3. 支付成功回调
4. 创建 `UserSubscription`
5. 将订单标记为成功

当前问题：

- 没有套餐级库存概念
- “购买上限”无法表达“全站总共还能卖多少份”
- 如果直接复用 `max_purchase_per_user`，会把“个人限购”和“套餐库存”混为一谈
- 现有订阅订单本身不知道自己是否持有库存锁，无法安全释放或转移锁定库存

用户已明确：

- `购买上限` 逻辑保持不变
- 单独新增“库存”功能
- 创建支付时先锁库存
- 支付成功后再扣减为已售
- 旧历史销售不追溯计入新库存
- 订阅过期、作废、删除都不回补已售库存
- 如果并发下出现问题，应优先用下单阶段锁库存来规避超卖

## Confirmed Decisions

以下内容已与用户确认：

- 保留 `max_purchase_per_user`，不改其原有语义
- 新增独立库存能力，而不是复用旧字段
- 库存限制与个人限购同时生效
- 创建支付订单时锁库存
- 支付成功时将锁定库存转为已售库存
- 支付失败、拉起支付失败、订单过期时释放锁定库存
- 订阅后续过期 / 作废 / 删除时不回补已售库存
- 老套餐历史销售不纳入新库存统计
- 后台手动新增订阅也应消耗库存

## Out of Scope

本次不做：

- 改变现有 `购买上限` 的语义或文案为库存
- 追溯历史订阅并自动回填已售库存
- 为库存增加预售、批次、仓位、SKU 等复杂模型
- 为库存不足后的支付成功回调实现自动退款流程
- 改造第三方支付平台侧的库存同步

## Approaches Considered

### Approach A: 复用现有 `max_purchase_per_user`

做法：

- 不新增字段
- 把现有“购买上限”直接改解释成“库存”

问题：

- 字段名、代码和 UI 现状都在表达“每用户限购”
- 会把套餐级限制和用户级限制混在一起
- 后续维护极易再次误解

### Approach B: 新增独立库存字段并显式维护计数（采用）

做法：

- 保留 `max_purchase_per_user`
- 新增库存相关字段
- 在订单和支付成功事务中维护锁定库存与已售库存

优点：

- 语义清晰
- 能同时表达“个人限购”和“套餐库存”
- 能准确支持“下单锁库存、支付成功转已售”的状态机

### Approach C: 库存只按实时订阅记录动态统计

做法：

- 不存 `stock_sold`
- 每次靠数订单或订阅记录得到剩余库存

问题：

- 用户已要求旧历史销售不追溯
- 管理员删除订阅后，实时统计会错误地把库存“补回来”
- 无法稳定表达“库存不回补”的业务语义

## Decision

采用 **Approach B**。

原因：

- 它是唯一能同时满足“购买上限不变”和“新增独立库存”的方案
- 它天然支持“待支付锁定库存”和“支付成功转已售”的双阶段流转
- 它能避免管理员删除订阅后，库存被错误回补的问题

## Data Model

在 `SubscriptionPlan` 上新增以下字段：

1. `stock_total`
   - 套餐总库存
   - `0` 表示不限库存
2. `stock_locked`
   - 已被待支付订单锁定、但尚未完成支付的库存
3. `stock_sold`
   - 已经成功售出的库存

在 `SubscriptionOrder` 上新增以下字段：

4. `stock_reserved`
   - 当前订单实际持有的库存锁份数
   - 默认 `0`
   - 当前版本固定只会是 `0` 或 `1`

约束与语义：

- 三个字段均为非负整数
- `stock_total = 0` 时：
  - 表示库存能力关闭
  - `stock_locked` 和 `stock_sold` 不参与购买限制判断
- 可下单库存：
  - `available_stock = stock_total - stock_locked - stock_sold`
- 允许下单条件：
  - `stock_total == 0`
  - 或 `available_stock > 0`

### Why Store Both `stock_locked` and `stock_sold`

必须显式保存这两个值，不能仅靠动态统计。

原因：

- `stock_locked` 用来表达待支付订单已占用的库存
- `stock_sold` 用来表达已经卖出的份数
- `stock_reserved` 用来表达某张订单是否真实占用了库存锁
- 用户明确要求老历史销售不回填库存
- 用户明确要求订阅删除后不回补库存
- 因此不能通过“实时数订阅记录”来推导库存

## Inventory Lifecycle

### 1. 创建支付订单

入口：

- 现有 `createPendingSubscriptionOrder`

同一事务内执行：

1. 锁定套餐行
2. 校验个人限购 `max_purchase_per_user`
3. 校验库存：
   - 若 `stock_total == 0`，跳过
   - 若 `stock_total > 0`，要求 `stock_total - stock_locked - stock_sold > 0`
4. 校验通过后：
   - `stock_locked += 1`
5. 创建 `SubscriptionOrder(status=pending)`，并记录：
   - `stock_reserved = 1`

结论：

- 只有成功创建待支付订单时，才会锁住一份库存
- 未成功创建订单，不会占库存
- 后续释放锁定库存或转为已售时，必须以订单上的 `stock_reserved` 为准

### 2. 拉起支付失败

现有流程中，第三方支付拉起失败后会调用订单过期逻辑。

本次需要在订单过期时释放锁定库存：

1. 锁定套餐行与订单行
2. 若订单仍是 `pending`
3. 若该订单 `stock_reserved > 0`，则：
   - `stock_locked -= 1`
   - `stock_reserved = 0`
4. 将订单置为 `expired`

### 3. 订单过期 / 手动关闭未支付订单

规则与“拉起支付失败”一致：

- 只要订单从 `pending` 走向终态失败
- 且该订单仍持有锁定库存
- 就必须释放：
  - `stock_locked -= 1`
  - `stock_reserved = 0`

### 4. 支付成功

入口：

- 现有 `CompleteSubscriptionOrder`

同一事务内执行：

1. 锁定订单行
2. 校验订单状态仍为 `pending`
3. 锁定套餐行
4. 防御性校验：
   - 若 `stock_total > 0`，要求 `stock_locked > 0`
   - 且订单 `stock_reserved > 0`
5. 创建 `UserSubscription`
6. 若 `stock_total > 0`：
   - `stock_locked -= stock_reserved`
   - `stock_sold += stock_reserved`
   - `stock_reserved = 0`
7. 订单标记为 `success`

### 5. 后续订阅状态变化

以下操作都不回补 `stock_sold`：

- 订阅过期
- 管理员作废订阅
- 管理员删除订阅

原因：

- 用户明确要求库存表示“卖出即消耗”
- 不是“当前活跃名额池”

## Old Plan Compatibility

本次不对历史销售做追溯。

兼容规则：

1. 迁移后老套餐默认：
   - `stock_total = 0`
   - `stock_locked = 0`
   - `stock_sold = 0`
2. 老套餐只有在管理员显式设置库存后，库存能力才开始生效
3. 设置库存前历史上已经卖出的订阅，不计入 `stock_sold`

### Re-enabling Stock

为了与“历史销售不追溯”保持一致，约定如下：

- 当某套餐 `stock_total` 从 `0` 改成正数时：
  - 视为开启一轮新的库存周期
  - 将 `stock_locked` 重置为 `0`
  - 将 `stock_sold` 重置为 `0`
- 当某套餐只是从一个正数改为另一个正数，例如 `50 -> 80`：
  - 不重置 `stock_locked`
  - 不重置 `stock_sold`

## Backend Changes

### Model

在 `SubscriptionPlan` 模型和数据库迁移中新增：

- `stock_total`
- `stock_locked`
- `stock_sold`

在 `SubscriptionOrder` 模型和数据库迁移中新增：

- `stock_reserved`

### Admin Create / Update Plan

管理端创建和编辑套餐时：

- 新增 `stock_total` 的校验
- 规则：
  - 不能为负数
  - `0` 表示不限库存
- 更新逻辑需区分：
  - `0 -> 正数`：重置 `stock_locked` / `stock_sold`
  - `正数 -> 正数`：仅更新 `stock_total`
  - `正数 -> 0`：关闭库存限制，但保留字段值或清零均可

额外限制：

- 若存在仍持有 `stock_reserved > 0` 的 pending 订单，则不允许执行会改变库存周期语义的切换：
  - `0 -> 正数`
  - `正数 -> 0`
- 若 `stock_total` 从一个正数改到另一个正数，新值不能小于：
  - `stock_locked + stock_sold`

建议：

- `正数 -> 0` 时清零 `stock_locked` 与 `stock_sold`
- 这样再次启用时语义最清晰

### Pending Order Creation

在 `createPendingSubscriptionOrder` 中新增：

- 套餐库存判断
- 套餐库存锁定

并保持与现有个人限购判断并列生效。

### Order Expiration

在 `ExpireSubscriptionOrder` 中新增：

- 如果订单仍为 `pending`
- 且订单 `stock_reserved > 0`
- 释放对应锁定库存
- 同时清零订单 `stock_reserved`

### Payment Completion

在 `CompleteSubscriptionOrder` 中新增：

- 将锁定库存转换为已售库存
- 转换依据必须是订单上的 `stock_reserved`

### Admin Manual Grant

管理员手动新增订阅时：

- 仍要校验个人限购
- 同时校验库存
- 直接消耗库存：
  - `stock_sold += 1`
- 不产生 `stock_locked`

## Concurrency Rules

库存相关判断必须全部在数据库事务内完成，并锁定套餐行。

### Goal

避免以下超卖场景：

- 套餐只剩 1 份库存
- 两个用户同时点击购买
- 两边都成功创建订单或都成功买到

### Rule

下单事务中使用套餐行锁：

1. 第一个事务拿到锁并检查可用库存
2. 若可下单，则先写入 `stock_locked += 1`
3. 第二个事务拿到锁时，看到剩余库存已减少
4. 若库存已满，则直接拒绝创建 pending 订单

结果：

- 大多数情况下，超卖问题在“创建订单阶段”就会被消除
- 而不是等到支付回调阶段才暴露
- 即使库存开关后来被管理员修改，也能根据订单 `stock_reserved` 做精确释放

## Error Handling

### User-Facing Errors

新增或复用以下业务错误：

- `库存不足`
- `已达到购买上限`
- `套餐未启用`
- `拉起支付失败`

当库存不足时：

- 用户端购买按钮应禁用
- 即使前端未及时刷新，后端事务仍必须返回库存不足错误

### Late Payment / Race Edge Case

理论上，创建订单时锁库存已经规避了绝大多数并发超卖。

但仍需防御以下异常：

- 订单已过期并释放锁定库存
- 第三方支付回调晚到
- 回调尝试把该订单完成为成功

处理方式：

- `CompleteSubscriptionOrder` 继续以订单状态为准
- 非 `pending` 状态直接拒绝
- 这类订单保持“需要人工处理/退款”的现有运维处理方式
- 本次不实现自动退款

## Frontend Changes

### Admin Edit Modal

管理端套餐编辑弹窗中：

1. 保留 `购买上限`
   - 文案改清楚：
     - `每个用户最多购买次数，0 表示不限`
2. 新增 `库存`
   - 文案：
     - `套餐总库存，0 表示不限`
3. 新增只读辅助信息：
   - `已售`
   - `锁定`
   - `剩余`

### Admin Subscription Table

套餐列表新增库存列。

推荐展示：

- 不限库存：
  - `不限`
- 启用库存：
  - `剩余 Z / 总库存 T`

必要时可在次级文案中展示：

- `已售 X，锁定 Y`

### User Purchase Card

用户端套餐卡片中：

1. 保留 `限购 N`
2. 新增 `剩余库存 N`
3. 当库存不限时，不展示库存标签
4. 当库存为 `0` 时：
   - 按钮禁用
   - 显示 `已售罄`

### User Purchase Modal

购买确认弹窗新增库存提示：

- `剩余库存：N`

当库存为 `0` 时：

- 所有支付按钮禁用

### Priority of Disabled Reasons

若同时满足：

- 用户个人限购达到上限
- 套餐库存已售罄

推荐优先展示：

- `已售罄`

原因：

- 这是全局不可购买状态
- 对用户解释更直接

## API Shape

现有套餐接口返回结构中新增以下字段：

- `stock_total`
- `stock_locked`
- `stock_sold`
- 可选：`stock_available`

推荐在后端响应里直接补一个只读派生字段：

- `stock_available = max(stock_total - stock_locked - stock_sold, 0)`

这样前端展示不需要复制业务计算。

## Migration Strategy

数据库迁移步骤：

1. 为 `subscription_plans` 新增列：
   - `stock_total`
   - `stock_locked`
   - `stock_sold`
2. 为 `subscription_orders` 新增列：
   - `stock_reserved`
3. 默认值全部为 `0`
4. 不做历史回填

这次迁移不需要扫描历史订单或历史订阅。

## Testing Strategy

### Model / Transaction Tests

至少覆盖：

1. `stock_total = 0` 时不限库存
2. 创建 pending 订单成功时：
   - `stock_locked +1`
3. 创建 pending 订单时库存不足：
   - 返回错误
4. 支付拉起失败 / 订单过期：
   - `stock_locked -1`
5. 支付成功：
   - `stock_locked -1`
   - `stock_sold +1`
   - `stock_reserved -> 0`
6. 管理员手动新增订阅：
   - `stock_sold +1`
7. 删除 / 作废 / 过期订阅：
   - `stock_sold` 不回退
8. 两个并发下单抢最后 1 份库存：
   - 只有一个成功创建 pending 订单
9. 有锁定订单存在时，库存开关切换或缩小库存总量会被拒绝

### Controller Tests

至少覆盖：

1. 创建套餐时可设置库存
2. 更新套餐时 `0 -> 正数` 会重置库存计数
3. 用户端获取套餐时返回库存字段
4. 库存为 0 时创建支付订单失败
5. 个人限购与库存可同时生效

### Frontend Tests

至少覆盖：

1. 管理端弹窗显示“购买上限”和“库存”两个独立字段
2. 套餐列表正确显示库存状态
3. 用户端套餐卡片库存售罄时按钮禁用
4. 购买弹窗库存售罄时支付按钮禁用
5. 同时存在个人限购和库存售罄时，优先显示库存售罄

## Risks

### Risk 1: 锁定库存泄漏

如果某些异常路径没有释放 `stock_locked`，会导致库存被永久卡住。

缓解：

- 所有 pending 失败终态统一收口到一个释放逻辑
- 为库存释放补充测试

### Risk 2: 订单状态与库存状态不一致

如果订单已成功但库存未转已售，或反之，会造成账实不符。

缓解：

- 库存变更与订单状态更新必须放在同一个事务里

### Risk 3: 管理员对“库存从现在开始算”的理解偏差

因为旧历史销售不回填，所以管理员可能误以为“库存 50”是总历史库存。

缓解：

- 在库存字段旁增加说明：
  - `库存从开启后开始统计，历史销售不计入`

## Summary

本次设计将订阅套餐的限制拆成两层：

- `购买上限`
  - 继续表示“每用户最多购买次数”
- `库存`
  - 表示“套餐总库存”

库存采用三字段模型：

- `stock_total`
- `stock_locked`
- `stock_sold`

并通过以下状态流转实现：

- 创建支付订单时锁库存
- 支付成功时转已售
- 未支付失败或过期时释放锁定
- 订阅后续生命周期变化不回补已售库存

这样既保留了原有限购能力，又新增了真正的套餐库存模型，而且不会与历史数据或管理员删除订阅的行为发生语义冲突。
