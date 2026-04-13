# Pricing Default Guest Group Design

## Goal

将模型广场对未登录用户的展示规则收敛为单一、可预测的产品语义，并与新用户注册默认分组保持一致。

本次目标是：

- 未登录用户访问模型广场时，固定按 `default` 分组展示模型
- 新注册用户默认分组明确为 `default`
- 已登录用户继续按自己的真实用户分组展示模型，不受影响
- 保留现有“需要登录访问”开关语义
- 不新增新的后台配置项
- 让管理员只需要围绕 `default` 分组管理公开模型范围

## Context

当前模型广场的展示逻辑分成两层：

1. 前端 `HeaderNavModules.pricing.enabled`
   - 控制顶栏里是否显示“模型广场”入口
2. 前端 `HeaderNavModules.pricing.requireAuth`
   - 控制 `/pricing` 页面是否需要登录

当 `requireAuth = false` 时，未登录用户可以直接访问 `/pricing`。但后端当前对未登录用户的处理是：

- 用户分组为空字符串 `""`
- 调用 `service.GetUserUsableGroups("")`
- 再用返回的 `usable_group` 与模型 `enable_groups` 做交集过滤

这导致“游客能看到哪些模型”取决于全局 `UserUsableGroups` 配置，而不是一个明确的“公开分组”概念。

当前实现存在几个问题：

- 管理员必须理解 `UserUsableGroups` 才知道游客能看到什么，产品心智不直观
- 游客展示范围和注册默认分组语义未统一
- 如果 `UserUsableGroups` 配得过宽，游客几乎等于能看全部模型
- 如果 `UserUsableGroups` 为空，游客虽然能进入模型广场，但最终看到空列表，体验不明确

同时，用户补充要求：

- 新注册用户默认为 `default` 分组

仓库现状中，`model.User.Group` 的数据库默认值已经是 `default`，但注册流程当前主要依赖数据库默认值隐式生效，没有把这条产品规则提升为显式、受测试保护的行为。

## Confirmed Product Decisions

以下内容已与用户确认：

- 采用固定方案，不新增 `AnonymousPricingGroup` 一类的新配置
- 未登录模型广场展示分组固定为 `default`
- `default` 作为公开基础分组的产品语义被正式采用
- 新注册用户默认分组为 `default`
- 已登录用户不做自动迁移，不强制改成 `default`
- “需要登录访问”打开时，未登录用户仍然不能访问模型广场

## Out of Scope

本次不做：

- 新增可配置的“游客展示分组”后台选项
- 修改 `UserUsableGroups` 在令牌分组选择中的现有语义
- 让未登录用户在系统其他业务流程中都被视为 `default` 用户
- 自动迁移历史老用户分组到 `default`
- 改造模型广场之外的接口访问权限模型

## Approaches Considered

### Approach A: 未登录模型广场固定按 `default` 展示（采用）

做法：

- 模型广场接口中，未登录时把空分组映射为 `default`
- 注册时明确保证新用户分组为 `default`
- 不新增配置

优点：

- 产品心智最简单
- 管理员不需要学习额外配置
- 游客公开展示和新用户基础分组统一

代价：

- `default` 被绑定为“公开基础分组”
- 将来如果想让游客看别的分组，需要改代码

### Approach B: 新增 `AnonymousPricingGroup` 配置

做法：

- 新增后台配置项，决定游客模型广场使用哪个分组

优点：

- 灵活度高
- 后续切换公开分组不用改代码

问题：

- 增加配置复杂度
- 当前需求明确选择简单固定方案，不需要这层可配置性

### Approach C: 保持现状，继续由 `UserUsableGroups` 决定游客可见范围

做法：

- 不改代码
- 继续通过 `UserUsableGroups` 配置游客可见模型

问题：

- 管理员心智复杂
- 游客公开范围不够显式
- 与“新注册用户默认 `default`”的产品语义不统一

## Decision

采用 **Approach A**。

一句话规则：

- `default` = 游客模型广场公开基础分组 + 新注册用户基础分组

## Target State

### 1. Guest Pricing Semantics

未登录用户访问模型广场时：

- 若 `HeaderNavModules.pricing.requireAuth = true`
  - 前端继续拦截并要求登录
- 若 `HeaderNavModules.pricing.requireAuth = false`
  - 后端将游客展示分组视为 `default`
  - 返回 `default` 分组对应可见模型

这里的“视为 `default`”仅用于模型广场展示上下文，不改变游客在其他接口中的身份语义。

### 2. Registered User Semantics

新注册用户创建成功后：

- 用户分组必须为 `default`
- 不再只依赖数据库 schema 默认值隐式生效
- 注册流程应显式保证该行为，并由测试保护

### 3. Authenticated User Semantics

已登录用户访问模型广场时：

- 继续使用真实 `user.group`
- 继续沿用现有用户分组可见规则
- 不回退到 `default`

## Backend Design

### 1. Pricing Context Resolution

在 `GetPricing` 中将“展示分组解析”拆成明确规则：

- 已登录：使用 `user.Group`
- 未登录：使用 `default`

推荐引入一个局部 helper 或清晰分支，避免继续把“空字符串用户分组”和“游客公开分组”混在一起。

### 2. Usable Group Calculation for Guests

游客模型广场展示不再以 `service.GetUserUsableGroups("")` 为核心语义。

目标行为是：

- 游客的展示上下文分组 = `default`
- 模型广场过滤结果 = `default` 分组可见的模型

实现层可以继续复用现有过滤函数，但输入语义必须转为显式 `default`。

### 3. Registration Flow

注册流程中应显式写入：

- `cleanUser.Group = "default"`

这样做的目的不是改变最终结果，而是：

- 让产品规则在应用层清晰可见
- 避免未来 schema 变更或 ORM 行为变化导致注册默认分组漂移
- 让测试能直接锁定这条规则

## Frontend / Admin UX

本次不新增新控件，但应补充清晰文案。

### 1. 模型广场设置文案

在“需要登录访问”附近补充说明：

- 关闭后，未登录用户可访问模型广场，并按 `default` 分组展示模型

### 2. 分组相关设置文案

在分组设置或相关帮助文案中，明确约定：

- `default` 为公开基础分组
- 想公开给游客和新注册用户使用的模型，应挂到 `default`

## Admin Operating Model

管理员今后应按以下方式理解模型公开范围：

- 想公开给游客和新注册用户的模型：挂到 `default`
- 只给高级用户看的模型：不要挂到 `default`，只挂到 `vip` / `svip` / 自定义分组
- 如果“需要登录访问”打开，则游客即使存在 `default` 公开模型，也无法访问模型广场

## Edge Cases

### 1. `default` 分组不存在

如果系统中不存在 `default`：

- 游客模型广场返回空列表
- 注册默认分组语义会失去依托

本次建议：

- 将 `default` 视为系统基础分组
- 在实现中至少保证：
  - 注册流程仍写入 `default`
  - 模型广场不会 panic 或报错
- 可选地在后台文案或日志中提示管理员：当前未配置 `default` 分组，游客将看不到模型

### 2. 模型不属于 `default`

若模型只属于 `vip` / `svip` / 自定义分组，则游客看不到。

### 3. 老用户分组

历史已存在用户保持原样：

- 不做批量迁移
- 不影响其模型广场展示逻辑

## Testing Strategy

### 1. 后端单元 / 控制器测试

至少覆盖：

- 未登录访问 `GetPricing` 时，按 `default` 分组返回模型
- 已登录访问 `GetPricing` 时，继续按真实用户分组返回模型
- `requireAuth` 逻辑不变（前端路由测试可覆盖）
- 注册新用户后，`user.group == "default"`
- 当 `default` 没有任何可见模型时，游客返回空列表而不是错误

### 2. 前端测试

至少覆盖：

- “需要登录访问”关闭时，文案正确表达“游客按 `default` 分组展示”
- “需要登录访问”开启时，`/pricing` 继续被登录路由保护

## Rollout Notes

本次为收敛语义的产品调整，不涉及数据迁移。

原因：

- `default` 已经是用户表和渠道表中的既有基础分组语义
- 新增的是“把游客模型广场展示也正式绑定到 `default`”
- 注册默认分组更多是显式化和测试化，而不是引入新数据模型

## Summary

本次设计将两条分散规则统一为一个明确产品语义：

- 游客看 `default`
- 新用户也是 `default`

这样管理员只需要经营好 `default` 分组，就能同时控制：

- 游客模型广场公开范围
- 新注册用户的基础分组起点

同时保留现有 `requireAuth` 开关，继续控制游客是否允许进入模型广场。
