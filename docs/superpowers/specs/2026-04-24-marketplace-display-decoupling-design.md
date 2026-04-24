# Marketplace Display Decoupling Design

## Goal

将“模型广场”的产品语义从“用户当前可用模型列表”调整为“站点对外展示的模型目录”。

本次目标是：

- `模型广场` 总开关只控制展示入口与展示页面，不影响真实调用能力
- 模型是否在模型广场展示，独立于真实调用能力配置
- 游客访问模型广场时，只展示 `default` 分组中的展示模型
- 已登录用户访问模型广场时，展示所有允许展示的模型，不再按用户可用分组裁剪
- 保留 `requireAuth` 语义，只控制游客是否允许访问模型广场
- 明确后台“模型管理”里的展示配置文案，避免管理员把“展示状态”和“调用状态”混淆

## Context

当前仓库里，模型广场数据主要来自 `/api/pricing`，并存在两套已经混在一起的语义：

1. `abilities.enabled`
   - 控制模型是否能被真实路由与调用
   - 来源于渠道与能力配置
2. `model_meta.status`
   - 当前实际上已经被 `model/pricing.go` 用来决定模型是否出现在模型广场
   - `status != 1` 时，该模型会被直接跳过，不返回给前端

同时，`/api/pricing` 在控制器层还会再按 `usable_group` 过滤一次展示结果。这个过滤逻辑适合“用户真实可用分组”语义，但不适合“模型广场只是展示目录”的新需求。

用户补充并确认的新要求是：

- 模型广场只和展示关联
- 游客只根据 `default` 分组可见模型
- 登录用户可以看到所有展示模型
- 模型是否真实可调用，继续由现有渠道、分组、能力开关控制

这意味着模型广场需要从“可用性视图”切换为“展示视图”。

## Confirmed Product Decisions

以下内容已与用户逐步确认：

- 采用“展示与调用解耦”方案，不让模型广场决定真实调用能力
- `HeaderNavModules.pricing.enabled` 只控制模型广场展示功能是否开放
- `HeaderNavModules.pricing.requireAuth` 只控制游客是否允许访问模型广场
- 游客只看 `default` 分组中的展示模型
- 登录用户看所有展示模型
- 模型广场里的分组筛选，对登录用户展示“全部展示分组”，对游客只展示 `default`
- 后台“模型管理”应继续作为模型广场展示配置入口

## Out of Scope

本次不做：

- 修改真实调用链路中的渠道选择、能力判定、分组授权逻辑
- 新增第二套“模型真实调用状态”字段
- 给登录用户做“是否可调用”的额外前端提示系统
- 改造模型广场之外的模型列表接口
- 自动迁移历史模型、分组或渠道数据
- 重构渠道管理界面

## Approaches Considered

### Approach A: 继续复用 `usable_group` 驱动模型广场

做法：

- 保持 `/api/pricing` 继续按 `usable_group` 过滤
- 仅补充游客 `default` 分组逻辑

优点：

- 改动最小

问题：

- 登录用户仍然只能看到自己可用分组里的模型
- 与“模型广场只做展示目录”的目标冲突
- 管理员仍然会把展示范围和实际可用范围混在一起理解

### Approach B: 新增独立“模型广场展示配置”，展示与调用完全解耦（采用）

做法：

- 模型广场总开关只控制展示入口、展示页面、展示接口
- 模型是否展示继续复用现有 `model_meta.status`
- `/api/pricing` 改成展示接口：
  - 游客只看 `default`
  - 登录用户看全部展示模型
- 前端不再用 `usable_group` 决定模型广场展示范围

优点：

- 产品语义清晰
- 后台配置入口统一
- 不影响真实调用能力
- 不需要新增数据库字段，因为现有 `model_meta.status` 已经承担展示语义

代价：

- 需要调整 `/api/pricing` 返回字段和前端消费方式
- 需要修改一部分现有文案，避免继续误导管理员

### Approach C: 新增新的数据库字段 `marketplace_visible`

做法：

- 不复用 `model_meta.status`
- 新增专门展示字段

优点：

- 命名更直观

问题：

- 仓库现状中 `model_meta.status` 已经在做这件事
- 新增字段会造成语义重叠和迁移成本
- 当前需求不需要承担这层复杂度

## Decision

采用 **Approach B**。

一句话规则：

- `模型广场 = 展示目录`
- `真实调用 = 渠道 + 分组 + abilities.enabled`
- `model_meta.status = 是否在模型广场展示`

## Target State

### 1. Marketplace Switch Semantics

`HeaderNavModules.pricing.enabled`：

- 只控制模型广场是否开放展示
- 关闭时：
  - 顶栏不显示“模型广场”入口
  - `/pricing` 页面不可见
  - `/api/pricing` 不返回模型广场展示数据
- 不影响任何真实调用能力

`HeaderNavModules.pricing.requireAuth`：

- 只在 `pricing.enabled = true` 的前提下生效
- 打开时，游客不能访问模型广场
- 关闭时，游客可以访问模型广场

### 2. Model Visibility Semantics

`model_meta.status` 的正式产品语义调整为：

- `status = 1`：允许在模型广场展示
- `status != 1`：不在模型广场展示

它不再被描述成“模型总状态”，而应明确为“模型广场展示状态”。

### 3. Guest Visibility Semantics

游客访问模型广场时：

- 仅返回 `status = 1` 的展示模型
- 且模型必须属于 `default` 分组
- 分组筛选只展示：
  - `all`
  - `default`

这里的 `default` 仅用于模型广场展示过滤，不意味着游客具备真实调用该分组模型的能力。

### 4. Authenticated User Visibility Semantics

已登录用户访问模型广场时：

- 返回所有 `status = 1` 的展示模型
- 不再按用户 `usable_group` 裁剪展示范围
- 分组筛选显示当前展示模型涉及到的全部分组

### 5. Real Usage Semantics

真实调用保持不变：

- 继续依赖渠道绑定、分组、能力、鉴权、额度等现有逻辑
- 继续由 `abilities.enabled` 和渠道状态控制
- 与模型广场展示完全解耦

因此允许以下状态同时存在：

- 模型可调用，但不在模型广场展示
- 模型在模型广场展示，但当前用户未必能真实调用

## API Design

### 1. `/api/pricing` Becomes a Display Endpoint

`GET /api/pricing` 的职责重新定义为：

- 返回“模型广场展示数据”
- 不再等同于“当前用户实际可用模型列表”

### 2. Response Contract

保留现有主要字段：

- `data`
- `vendors`
- `group_ratio`
- `supported_endpoint`
- `pricing_version`

新增或调整字段语义：

- `display_groups`
  - 新增
  - 模型广场分组筛选应基于它渲染
  - 游客：只包含 `default`
  - 登录用户：包含所有展示模型涉及到的分组

- `group_ratio`
  - 保留字段名
  - 语义调整为“展示分组倍率”
  - 只包含 `display_groups` 中出现的分组倍率

- `usable_group`
  - 短期可保留以兼容其他潜在消费端
  - 但模型广场前端不再依赖它做展示过滤

- `auto_groups`
  - 可暂时保留响应字段以避免兼容风险
  - 但模型广场前端不再把它作为展示语义的一部分

### 3. Disabled Marketplace Response

当 `pricing.enabled = false` 时，`/api/pricing` 返回稳定空结果：

- `success = true`
- `data = []`
- `vendors = []`
- `group_ratio = {}`
- `display_groups = {}`
- 其余展示相关字段为空

选择稳定空结果而不是报错，原因是：

- 兼容现有前端加载逻辑
- 避免把“功能关闭”误判成“接口异常”
- 防止直连接口泄露展示数据

## Backend Design

### 1. HeaderNavModules Runtime Parsing in Go

后端需要增加与前端一致的 `HeaderNavModules.pricing` 解析逻辑：

- `enabled` 默认 `true`
- `requireAuth` 默认 `false`
- 兼容旧格式 `pricing: boolean`

目的：

- 让 `/api/pricing` 在后端也能基于真实配置进行展示兜底
- 避免只靠前端隐藏入口导致直连接口仍能看到模型数据

### 2. Pricing Filtering Rules

控制器层将展示过滤改成两段：

1. 先按模型广场总开关决定是否直接返回空展示结果
2. 再按访问身份决定展示范围

建议顺序：

1. 读取原始 `pricing := model.GetPricing()`
2. 若模型广场关闭，直接返回空展示结果
3. 若开启：
   - 游客：仅保留 `enable_groups` 包含 `default` 的模型
   - 登录用户：不过滤分组，仅保留 `GetPricing()` 已经产出的展示模型

注意：

- `GetPricing()` 内部已经依据 `model_meta.status` 排除了不应展示的模型
- 因此控制器不需要再次处理模型展示状态

### 3. Display Groups Construction

后端应根据最终返回给前端的 `data` 构造 `display_groups`：

- 从最终展示模型的 `enable_groups` 中收集所有分组
- 游客仅保留 `default`
- 登录用户保留全部出现过的分组

这个集合是模型广场筛选 UI 的唯一来源。

### 4. Group Ratio Construction

`group_ratio` 也应基于 `display_groups` 重新裁剪：

- 游客：仅返回 `default`
- 登录用户：返回全部展示分组对应倍率

这样前端看到的分组按钮与倍率信息保持一致，不再依赖真实可用分组。

## Frontend Design

### 1. Route Gating

前端需要把 `pricing.enabled` 传到公共路由层：

- 顶栏导航根据 `pricing.enabled` 决定是否显示“模型广场”
- `/pricing` 路由在 `pricing.enabled = false` 时直接不可见

推荐行为：

- 渲染 `NotFound`

原因：

- 表示功能已关闭，而不是权限不足

### 2. Pricing Page Data Consumption

模型广场前端从“可用分组视图”切换到“展示分组视图”：

- 分组按钮改用 `display_groups`
- 登录用户看到全部展示分组
- 游客只看到 `default`

### 3. Terminology Update

当前界面中的“可用令牌分组”文案会误导用户以为这些分组等于真实权限。

建议改成：

- `模型分组`
  或
- `展示分组`

推荐使用：

- `模型分组`

原因：

- 简洁
- 不暗示真实调用能力

### 4. Model Detail Side Sheet

模型详情中的“分组价格”表应按展示语义渲染：

- 用 `display_groups` 而不是 `usable_group`
- 游客只看到 `default`
- 登录用户看到该模型涉及到的全部展示分组

`auto 分组调用链路` 建议从模型广场详情中移除，原因是：

- 它属于真实调用路由语义
- 与“模型广场只负责展示”的目标冲突

### 5. Admin Model Management UX

后台“模型管理”界面应明确这是模型广场展示控制，不是调用控制：

- 编辑弹窗中的 `状态` 改名为 `模型广场展示`
- 列表列名改成 `广场展示`
- 保留现有 warning：
  - 这里只影响模型广场展示，不影响真实调用与路由

这可以把现有隐藏语义显式化，降低管理员误操作概率。

## Implementation Areas

预计涉及的文件范围：

- `controller/pricing.go`
- `web/src/App.jsx`
- `web/src/routes/PublicRoutes.jsx`
- `web/src/hooks/model-pricing/useModelPricingData.jsx`
- `web/src/components/table/model-pricing/filter/PricingGroups.jsx`
- `web/src/components/table/model-pricing/modal/components/ModelPricingTable.jsx`
- `web/src/components/table/models/modals/EditModelModal.jsx`
- `web/src/components/table/models/ModelsColumnDefs.jsx`
- 必要时新增一个 Go 侧的 `HeaderNavModules` 解析 helper

## Testing Strategy

### Backend Tests

至少覆盖以下场景：

1. 模型广场关闭时，`/api/pricing` 返回空展示结果
2. 游客访问时，只返回 `default` 分组中的展示模型
3. 登录用户访问时，返回所有展示模型，不按用户可用分组过滤
4. `model_meta.status = 0` 的模型不会出现在模型广场
5. 即使模型不在模型广场展示，只要 `abilities.enabled = true`，真实调用能力语义不受本次改动影响

### Frontend Tests

至少覆盖以下场景：

1. `pricing.enabled = false` 时，导航不显示“模型广场”
2. `pricing.enabled = false` 时，`/pricing` 不可见
3. 模型广场分组组件改为读取 `display_groups`
4. 模型管理弹窗和列表使用新的展示文案

## Risks

### 1. 兼容风险

如果直接删除 `usable_group` / `auto_groups`，可能影响未发现的消费端。

缓解方式：

- 本次先保留字段
- 前端改为只消费 `display_groups`

### 2. 语义切换风险

管理员可能已经习惯“模型广场里看到的模型 = 当前用户真实能用的模型”。

缓解方式：

- 明确改文案
- 在模型管理页保留 warning 提示

### 3. 前后端字段切换风险

如果前端仍有局部组件继续使用 `usable_group`，会出现展示不一致。

缓解方式：

- 统一搜索 `usableGroup` 与 `usable_group`
- 一次性改完模型广场页、筛选器、详情面板

## Rollout Notes

该设计不需要数据库迁移，因为：

- `model_meta.status` 已经存在
- 当前实现中它已经承担“是否在模型广场展示”的实际行为

因此本次主要是：

- 明确语义
- 修正过滤逻辑
- 修正文案
- 补充测试

## Final Product Rule

最终产品规则总结如下：

- 模型广场是展示目录，不是权限目录
- 游客只看 `default` 分组中的展示模型
- 登录用户看全部展示模型
- 模型是否展示由模型管理中的“模型广场展示”控制
- 模型是否真实可调用继续由渠道与能力配置控制
