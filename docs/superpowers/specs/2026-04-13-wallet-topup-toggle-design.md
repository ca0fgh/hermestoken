# 钱包管理额度充值开关设计

## 1. 背景

当前 `/console/topup` 页面在存在订阅套餐时，会同时展示：

- `订阅套餐`
- `额度充值`

同时，个人设置页已经存在“钱包管理”相关的用户设置语义，但现有实现只覆盖侧边栏入口显示，不覆盖钱包管理页内部的功能显隐。

本次需求是在“钱包管理”下新增一个用户级开关，用来控制钱包管理页内是否显示 `额度充值`。

## 2. 目标

- 在个人设置中提供一个用户级开关控制 `额度充值`
- 默认开启，兼容现有所有用户
- 关闭后仅隐藏钱包管理页内的 `额度充值`，不影响 `钱包管理` 入口
- 不影响 `订阅套餐`、账单、兑换码充值及现有订阅扣费逻辑

## 3. 非目标

- 不修改侧边栏模块权限模型
- 不复用 `sidebar_modules.personal.topup` 作为页内功能开关
- 不新增管理员级全局配置
- 不调整充值支付网关、订阅购买、账单记录的业务逻辑

## 4. 用户体验

### 4.1 个人设置页

在个人设置页“其他设置”中新增一项：

- 标题：`钱包管理`
- 描述：`余额充值管理`
- 控件：开关

交互规则：

- 默认显示为开启
- 用户切换后点击“保存设置”才真正持久化
- 保存成功后维持现有成功提示与刷新逻辑

### 4.2 钱包管理页

路径：`/console/topup`

展示规则：

- 当用户开关为开启：
  - 保持现状
  - 若存在订阅套餐，显示 `订阅套餐` 和 `额度充值` 两个 tab
  - 若不存在订阅套餐，直接展示充值内容
- 当用户开关为关闭：
  - 不显示 `额度充值` tab
  - 不显示在线充值表单和对应充值内容
  - 保留页面卡片头、账单按钮、订阅套餐区域、兑换码充值区域
  - 若没有订阅套餐，也不回退显示充值内容，只保留非额度充值内容

## 5. 数据设计

### 5.1 用户设置字段

在 `dto.UserSetting` 中新增字段：

- `QuotaTopupEnabled *bool` 对应 JSON key `quota_topup_enabled`

使用指针布尔而不是普通 `bool`，原因：

- `nil` 代表“用户从未设置”，此时按默认开启处理
- `false` 代表用户显式关闭，必须被持久化
- 可避免 `omitempty` 导致显式 `false` 无法写入 JSON

### 5.2 默认值策略

统一默认规则：

- `setting.quota_topup_enabled == nil` 时，视为开启
- `setting.quota_topup_enabled == false` 时，视为关闭

前后端都采用同一默认语义，避免刷新前后状态不一致。

## 6. 后端设计

### 6.1 读取

`GET /api/user/self`

现有接口已经返回：

- `setting`
- `sidebar_modules`

本次不需要新增独立顶层字段，只要保证 `setting` 中包含 `quota_topup_enabled` 即可。前端继续从用户 setting 中读取该开关。

### 6.2 保存

`PUT /api/user/setting`

扩展现有用户设置保存逻辑，支持接收：

- `quota_topup_enabled`

保存规则：

- 读取当前 `UserSetting`
- 仅更新本次提交字段
- 其余设置保持不变

### 6.3 `/api/user/self` 特殊更新逻辑

`PUT /api/user/self` 当前只特殊处理：

- `sidebar_modules`
- `language`

本需求不走该接口保存开关，避免把“个人资料更新”和“用户设置更新”再次耦合。

## 7. 前端设计

### 7.1 个人设置状态

`PersonalSetting.jsx`

扩展 `notificationSettings` 本地状态，新增：

- `quotaTopupEnabled: true`

初始化规则：

- 从 `userState.user.setting` 读取 `quota_topup_enabled`
- 若字段不存在，则回填 `true`

保存规则：

- 调用 `/api/user/setting` 时带上 `quota_topup_enabled`

### 7.2 设置 UI

`NotificationSettings.jsx`

在“其他设置”卡片中新增单独的“钱包管理”tab，避免把该开关混入通知、价格、隐私或边栏设置中。

tab 内容只包含一项卡片式开关：

- 标题：`钱包管理`
- 描述：`余额充值管理`
- 开关字段：`quotaTopupEnabled`

理由：

- 与截图中的视觉语义一致
- 和“边栏设置”的菜单显隐能力区分清楚
- 未来如果钱包管理下继续增加“账单显示”“兑换码充值”等子项，也能继续扩展

### 7.3 钱包管理页显隐

`TopUp/index.jsx`

新增用户偏好读取逻辑：

- 从 `/api/user/self` 返回的 `setting` 解析 `quota_topup_enabled`
- 解析不到时默认 `true`

`RechargeCard.jsx`

新增 prop：

- `quotaTopupEnabled`

渲染规则：

- `shouldShowSubscription && quotaTopupEnabled`：
  - 显示两个 tab：`订阅套餐`、`额度充值`
- `shouldShowSubscription && !quotaTopupEnabled`：
  - 只显示 `订阅套餐`
- `!shouldShowSubscription && quotaTopupEnabled`：
  - 保持现状，直接显示充值内容
- `!shouldShowSubscription && !quotaTopupEnabled`：
  - 不显示充值内容，改为仅显示兑换码充值等非额度充值内容

实现上应将现有内容拆成：

- `topupInteractiveContent`：额度充值主体
- `redeemCodeContent`：兑换码充值
- `sharedContainer`：卡片框架

避免用大量条件分支重复整块 JSX。

## 8. 兼容性与风险

### 8.1 兼容现有用户

旧用户 `setting` 中没有 `quota_topup_enabled`，默认按开启处理，不需要迁移脚本。

### 8.2 风险点

- 若仍使用普通 `bool + omitempty`，显式关闭可能不会写入 JSON
- 若前端默认值与后端默认值不一致，会出现刷新后开关跳变
- 若把该能力错误复用到 `sidebar_modules.personal.topup`，会导致菜单入口和页内内容联动，偏离需求

## 9. 测试设计

### 9.1 后端测试

覆盖以下场景：

- `UserSetting` 在 `quota_topup_enabled == nil` 时序列化/反序列化后仍按默认开启解释
- `quota_topup_enabled == false` 时可被正确保存，不会因 JSON tag 丢失
- 用户设置更新接口写入该字段后，重新读取 `setting` 能看到显式 `false`

### 9.2 前端测试

覆盖以下场景：

- 当 `quota_topup_enabled` 未设置时，页面仍展示 `额度充值`
- 当 `quota_topup_enabled` 为 `false` 时，`RechargeCard` 不再渲染 `额度充值`
- 当有订阅套餐且开关关闭时，`订阅套餐` 仍可见

## 10. 实施范围

预计修改文件：

- `dto/user_settings.go`
- `controller/user.go`
- `web/src/components/settings/PersonalSetting.jsx`
- `web/src/components/settings/personal/cards/NotificationSettings.jsx`
- `web/src/components/topup/index.jsx`
- `web/src/components/topup/RechargeCard.jsx`
- 对应前后端测试文件

## 11. 验收标准

- 用户可在个人设置中关闭“额度充值”
- 关闭后刷新页面仍保持关闭状态
- `/console/topup` 页面不再显示 `额度充值`
- `订阅套餐`、账单、兑换码充值不受影响
- 未配置该字段的旧用户行为保持不变
