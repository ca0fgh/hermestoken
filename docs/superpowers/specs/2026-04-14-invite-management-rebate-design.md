# Invite Management Rebate Design

## Goal

为 `hermestoken` 新增一个面向邀请人的自助“邀请管理”模块，用于集中管理邀请返佣。

本次目标是：

- 在侧边栏新增与 `个人中心` 同级的 `邀请管理` 分组
- 在 `邀请管理` 下新增 `邀请返佣` 子菜单
- 让邀请人可以查看自己名下所有被邀请人
- 展示每个被邀请人给当前邀请人带来的累计返佣贡献
- 在同一页内同时编辑：
  - 邀请人的默认返佣规则
  - 某个被邀请人的独立返佣覆盖
- 第一版只支持 `订阅返佣`
- 规则维度与现有管理端保持一致：
  - `返佣类型 + 分组 + 返佣率`

## Context

当前项目已经具备以下返佣基础：

1. 管理员配置层
   - 管理端用户编辑弹窗已有“邀请人返佣覆盖”
   - 逻辑已经演进为可扩展覆盖项列表
   - 当前数据维度是：
     - `返佣类型`
     - `分组`
     - `覆盖总返佣率`
   - 当前真实支持的返佣类型只有 `订阅返佣`

2. 邀请人默认层
   - 用户端 `钱包管理` 中已有邀请奖励卡片
   - 邀请人当前可以配置“默认分给被邀请人的订阅返佣比例”
   - 默认规则目前落在：
     - `user.setting.subscription_referral_invitee_rate_bps_by_group`

3. 返佣结算与账本层
   - 订阅返佣记录落在：
     - `subscription_referral_records`
   - 每笔记录包含：
     - `payer_user_id`
     - `inviter_user_id`
     - `beneficiary_role`
     - `reward_quota`
     - `reversed_quota`
     - `debt_quota`
   - 已存在分组级订阅返佣结算逻辑

4. 现有导航结构
   - 用户侧当前有 `个人中心` 分组
   - 其下现有 `钱包管理`、`个人设置`
   - 本次用户已确认：
     - `邀请管理` 必须与 `个人中心` 同级
     - 不是挂在 `个人中心` 下面

## Confirmed Product Decisions

以下内容已与用户确认：

- 侧边栏新增一个独立父级模块 `邀请管理`
- `邀请管理` 与 `个人中心` 同级
- `邀请管理` 下先只有一个子菜单：`邀请返佣`
- 页面结构采用单页方案，不拆详情页
- 页面主布局采用：
  - 顶部统计
  - 默认返佣规则区
  - 下方左侧被邀请人列表
  - 下方右侧当前选中被邀请人的独立返佣配置
- “被邀请人的贡献”口径为：
  - 该被邀请人累计给当前邀请人带来的返佣收益
- 顶部统计只保留：
  - `被邀请人数`
  - `累计返佣收益`
- 不展示“当前启用独立返佣人数”
- 列表采用简单分页
- 支持按 `用户 ID / 用户名` 搜索
- 页面内需要同时支持编辑“默认规则”
- 第一版只支持 `订阅返佣`
- 被邀请人独立返佣也必须遵循现有逻辑：
  - `被邀请人 + 返佣类型 + 分组 -> 独立返佣率`

## Out of Scope

本次不做：

- 第二种真实返佣类型的后端实现
- 支持按套餐维度单独配置被邀请人返佣
- 排行榜、趋势图、复杂分析报表
- 复杂筛选器，例如贡献区间、活跃时间区间
- 将现有 `钱包管理` 邀请卡片完全删除
- 修改管理员覆盖规则的业务语义
- 重做返佣结算公式

## Approaches Considered

### Approach A: 单页双区管理页（采用）

结构：

- 顶部统计区
- 默认返佣规则区
- 左侧被邀请人列表
- 右侧当前选中人的独立返佣配置区

优点：

- 默认规则与独立覆盖关系最清楚
- 与当前“管理员覆盖 -> 邀请人默认 -> 被邀请人独立”三层逻辑完全对齐
- 后续增加返佣类型时，只需要扩展两个配置区，不需要重做页面骨架
- 比拆详情页更高效

缺点：

- 页面信息密度较高
- 需要更谨慎地控制左右区域的视觉层级

### Approach B: 双 Tab 页面

结构：

- Tab 1：默认返佣规则
- Tab 2：被邀请人独立返佣

问题：

- 默认规则与独立覆盖关系被切断
- 用户必须来回切换页面状态
- 不利于快速比对默认值与独立值

### Approach C: 列表页 + 详情页

结构：

- 第一页只显示被邀请人列表
- 详情页单独管理某个被邀请人的返佣

问题：

- 第一版过重
- 操作路径更深
- 不符合用户“在一个管理页里同时看默认规则和独立返佣”的确认结果

## Decision

采用 **Approach A**。

原因：

- 这是唯一一个既满足“新独立模块导航”，又满足“单页内同时编辑默认规则和独立返佣”的方案
- 它最容易复用现有返佣字段和接口边界
- 它为未来新增返佣类型保留了扩展空间，但不强迫本次实现超出范围

## Information Architecture

### Sidebar

新增一个新的用户侧分组：

- 分组标题：`邀请管理`
- 子菜单：
  - `邀请返佣`

调整后用户侧结构为：

- `控制台`
- `邀请管理`
  - `邀请返佣`
- `个人中心`
  - `钱包管理`
  - `个人设置`

### Route

新增独立路由：

- `GET /console/invite/rebate`

说明：

- 不复用 `TopUp` 页面路由
- 不挂在 `PersonalSetting` 页面内部
- 使用独立页面组件承载整个邀请返佣管理

### Sidebar Settings Integration

现有 `sidebar_modules` 需要增加新的 section：

- section key：`invite`
- module key：`rebate`

这意味着以下位置都要同步支持新 section：

- 后端默认侧边栏配置生成
- 前端 `useSidebar` 默认结构
- 个人设置中的侧边栏显隐设置
- 权限检查中的 section/module 解析

## Target UX

### Page Layout

页面采用三段式结构：

1. 顶部统计区
2. 默认返佣规则区
3. 被邀请人管理区

其中“被邀请人管理区”内部采用左右分栏：

- 左侧：被邀请人列表
- 右侧：当前选中人的独立返佣配置

### Top Summary

只展示两个指标：

1. `被邀请人数`
2. `累计返佣收益`

口径：

- `被邀请人数`
  - 当前用户名下 `inviter_id = 当前用户 id` 的人数
- `累计返佣收益`
  - 当前用户作为邀请人所获得的净订阅返佣收益总和

### Default Rule Section

该区域用于编辑邀请人的默认返佣规则。

第一版保持可扩展列表结构，每条规则包含：

1. `返佣类型`
2. `分组`
3. `默认给被邀请人的返佣率`

当前交互规则：

- 返佣类型下拉固定只有 `订阅返佣`
- 分组只显示当前邀请人已启用订阅返佣的分组
- 默认返佣率输入为百分比，范围 `0 ~ 100`
- 默认返佣率最终不能超过该分组对当前邀请人生效的总返佣率
- 用户可以新增规则、修改规则、删除规则
- 如果删除某个默认规则，则该分组回退为“未配置”

该区域的核心语义是：

- 这是“面向所有被邀请人”的默认值
- 只有没有独立覆盖的被邀请人会使用它

### Invitee List

列表位于页面左侧，支持：

- 分页
- 关键字搜索

搜索范围：

- 用户 ID
- 用户名

列表字段：

1. `用户`
   - 用户 ID
   - 用户名
2. `当前分组`
3. `累计贡献`
4. `独立返佣`
   - `未设置`
   - 或 `已设置 N 项`

列表不做内联编辑。

点击某行后：

- 右侧配置区切换到该被邀请人
- 当前行高亮

### Invitee Override Section

右侧展示当前选中被邀请人的独立返佣配置。

空状态：

- 如果尚未选中被邀请人，则显示“请选择一个被邀请人”

配置模型与默认规则保持一致，每条配置项包含：

1. `返佣类型`
2. `分组`
3. `独立返佣率`

交互规则：

- 第一版返佣类型固定只有 `订阅返佣`
- 分组只显示当前邀请人已启用订阅返佣的分组
- 可新增、修改、删除独立规则
- 未设置独立规则时，回退到页面上方的默认规则
- 右侧应显示弱化提示，帮助用户理解回退关系，例如：
  - `未设置独立返佣时，使用默认规则`
  - `当前默认返佣率 15%`

## Effective Priority

对于某个被邀请人 `invitee`、返佣类型 `subscription`、分组 `G`：

1. 先读取该被邀请人的独立返佣率
2. 如果未配置，则读取邀请人的默认返佣率
3. 最终结果裁剪到该分组对邀请人生效的总返佣率以内
4. 如果该分组对邀请人的总返佣率 `<= 0`，则该分组视为未启用，不允许保存默认或独立返佣

统一公式：

- `effectiveInviteeRate = min(overrideOrDefaultRate, effectiveTotalRate)`

## Contribution Definition

“被邀请人的累计贡献”定义为：

- 当前邀请人作为 `inviter`
- 当前被邀请人作为该笔订阅的 `payer_user_id`
- 且当前返佣记录的 `beneficiary_role = inviter`
- 对这些记录聚合后的净收益

净收益公式：

- `net_reward_quota = reward_quota - reversed_quota - debt_quota`

说明：

- 这样可以避免退款或冲正后列表仍然高估贡献
- 顶部的 `累计返佣收益` 也应使用同一口径聚合

## Data Model

### Existing Default Rule Storage

默认规则继续复用现有用户设置：

- `dto.UserSetting.subscription_referral_invitee_rate_bps_by_group`

这意味着：

- 本次不新增新的“默认规则表”
- 页面顶部默认规则区只是把现有 map 适配成可编辑列表

### New Invitee Override Table

新增一张专用表，用于存放“邀请人针对某个被邀请人的独立订阅返佣覆盖”。

建议模型：

```go
type SubscriptionReferralInviteeOverride struct {
    Id            int
    InviterUserId int
    InviteeUserId int
    Group         string
    InviteeRateBps int
    CreatedAt     int64
    UpdatedAt     int64
}
```

唯一键：

- `(inviter_user_id, invitee_user_id, group)`

说明：

- 第一版只做订阅返佣，所以不额外加 `type` 列
- 前端状态模型仍保留 `type` 字段，方便未来扩展
- 当前持久化表只存 `subscription`

### Frontend Row Model

默认规则区和独立返佣区都统一使用可扩展行模型：

```js
{
  id,
  type,
  group,
  inputPercent,
  effectiveTotalRateBps,
  isDraft,
  hasOverride
}
```

其中：

- `type`
  - 当前固定为 `subscription`
- `group`
  - 命中的返佣分组
- `inputPercent`
  - 用户正在编辑的百分比值
- `effectiveTotalRateBps`
  - 当前邀请人在该分组生效的总返佣率，仅用于提示和校验
- `hasOverride`
  - 表示该行是否已落库或已存在于默认 map 中

## API Design

### Reuse Existing Default Rule API

默认规则继续复用现有接口：

- `GET /api/user/referral/subscription`
- `PUT /api/user/referral/subscription`
- `DELETE /api/user/referral/subscription?group=vip`

职责：

- `GET`
  - 返回顶部统计所需的返佣汇总
  - 返回默认规则区所需的分组和默认返佣值
- `PUT`
  - 更新某个分组的默认返佣率
- `DELETE`
  - 删除某个分组的默认返佣率，使其回退为“未配置”

说明：

- 本次明确采用 `DELETE` 协议删除默认规则
- 不使用 `invitee_rate_bps = 0` 兼作删除语义，避免前后端含义混淆

### New Invitee List API

新增邀请管理列表接口：

- `GET /api/user/referral/subscription/invitees`

查询参数：

- `page`
- `page_size`
- `keyword`

返回内容：

- 分页列表
- 顶部统计：
  - `invitee_count`
  - `total_contribution_quota`

列表项字段建议：

```json
{
  "id": 12,
  "username": "alice",
  "group": "vip",
  "contribution_quota": 150000,
  "override_group_count": 2
}
```

### New Invitee Detail API

新增当前选中被邀请人的详情接口：

- `GET /api/user/referral/subscription/invitees/:invitee_id`

返回内容：

- invitee 基础信息
- 当前邀请人可用的分组列表
- 该 invitee 已保存的独立返佣规则列表
- 每个分组对应的默认返佣率
- 每个分组对应的当前总返佣率

该接口的目标是让右侧面板一次拿齐渲染所需信息，避免前端自行拼装过多来源。

### New Invitee Override Mutation APIs

新增独立返佣写接口：

- `PUT /api/user/referral/subscription/invitees/:invitee_id`
- `DELETE /api/user/referral/subscription/invitees/:invitee_id?group=vip`

`PUT` 请求体：

```json
{
  "group": "vip",
  "invitee_rate_bps": 1500
}
```

规则：

- 只能修改当前用户自己邀请的被邀请人
- 分组必须属于当前邀请人已启用的订阅返佣分组
- `invitee_rate_bps` 必须在 `0 ~ effective_total_rate_bps` 之间

## Permission and Validation Rules

### Ownership Validation

所有新接口都必须验证：

- `invitee.InviterId == currentUserId`

如果不满足：

- 返回权限错误或资源不存在

### Group Validation

所有可编辑分组都必须满足：

- 是合法订阅返佣分组
- 当前邀请人在该分组有生效总返佣率
- 当前生效总返佣率 `> 0`

### Rate Validation

默认规则与独立返佣统一校验：

1. 必须是数字
2. 必须在 `0 ~ 100` 之间
3. 转换成 `bps` 后不得超过该分组当前总返佣率

## Implementation Notes

### Backend

建议新增或扩展以下能力：

- `model` 层新增 invitee override 表与 CRUD
- `model` 层新增“当前邀请人名下 invitee 列表 + 贡献聚合”查询
- `controller/subscription_referral.go` 继续承载订阅返佣自助接口
- `router/api-router.go` 在 `selfRoute` 下新增 invitee 自助管理子路由

### Frontend

建议新增独立页面和组件，而不是把逻辑塞回 `TopUp`：

- 新页面：
  - `web/src/pages/InviteRebate`
- 新组件目录：
  - `web/src/components/invite-rebate/...`

推荐拆分：

- `InviteRebatePage`
- `InviteRebateSummary`
- `InviteDefaultRuleSection`
- `InviteeListPanel`
- `InviteeOverridePanel`
- `inviteRebate` helper / parser

### Existing Wallet Management

`钱包管理` 中现有邀请奖励卡片先保留。

本次不要求立即下线，但要避免两处编辑逻辑长期分叉。

因此建议：

- 让新页面成为默认管理入口
- 后续再决定是否将钱包页中的默认规则编辑收敛为只读摘要

## Testing Strategy

### Backend Tests

新增测试覆盖：

1. 邀请人只能看到自己的被邀请人
2. 分页和关键词搜索正确
3. 贡献聚合口径正确，且能扣除冲正与 debt
4. 只能编辑自己名下 invitee
5. 非法分组拒绝
6. 超过当前总返佣率拒绝
7. 删除独立覆盖后回退到默认规则

### Frontend Tests

新增测试覆盖：

1. 侧边栏出现新的 `邀请管理 -> 邀请返佣`
2. 页面布局包含：
   - 顶部统计
   - 默认规则区
   - 左侧 invitee 列表
   - 右侧 override 面板
3. 列表搜索和分页参数传递正确
4. 默认规则区继续使用 `返佣类型 + 分组 + 返佣率`
5. 选中 invitee 后右侧正确渲染已保存独立规则
6. 未选中 invitee 时显示空状态
7. 保存/删除独立返佣调用正确接口

## Risks

1. 默认规则和钱包页现有逻辑重复
   - 必须明确谁是主入口，避免未来双向漂移

2. 贡献聚合口径如果直接取 `aff_history`
   - 会无法正确处理冲正
   - 必须基于 `subscription_referral_records` 聚合

3. invitee override 若不做所有权校验
   - 会造成越权查看和编辑

4. 页面如果把默认规则和独立规则都做成大表格
   - 会损失层级清晰度
   - 需要坚持“上默认、下分栏”的布局原则
