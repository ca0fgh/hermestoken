# 管理员支付配置说明

本文说明 `http://localhost:3000/console/setting?tab=payment` 对应的管理员支付配置页面如何工作、每个字段保存到哪里，以及最终如何影响用户侧充值和订阅支付。

适用项目：`subproject/hermestoken`

## 1. 页面入口

- 管理员设置页 `payment` 标签入口在 `web/src/pages/Setting/index.jsx`
- 支付配置聚合页在 `web/src/components/settings/PaymentSetting.jsx`
- 页面分为 5 个区块：
  - 通用设置
  - 易支付/EPay
  - Stripe
  - Creem
  - Waffo

## 2. 配置保存方式

这个页面不是把所有支付配置保存成一个大 JSON，而是把每个字段拆成独立的 `option key`。

### 2.1 读取流程

1. 前端调用 `GET /api/option/`
2. 后端从 `common.OptionMap` 返回所有配置
3. 名称以 `Token`、`Secret`、`Key` 等结尾的敏感字段不会回传到前端

这意味着：

- 管理员重新打开页面时，很多密钥输入框会显示为空
- 输入框空白不代表数据库里没有值

### 2.2 保存流程

1. 前端按字段逐个调用 `PUT /api/option/`
2. 后端在 `model.UpdateOption` 中先写数据库
3. 然后同步更新 `common.OptionMap` 和对应的运行时变量

因此，大多数支付配置保存后会立即生效，不需要重启服务。

## 3. 通用设置

通用设置只有一个字段：

| 页面字段 | Option Key | 作用 |
|---|---|---|
| 服务器地址 | `ServerAddress` | 影响默认回调地址、默认支付完成跳转地址、部分控制台链接 |

说明：

- 推荐填写最终对外访问的完整域名，例如 `https://api.example.com`
- Stripe 页面保存前会校验 `ServerAddress` 是否已填写
- 如果没有单独设置回调地址，系统会默认使用 `ServerAddress`

## 4. 易支付/EPay 配置

对应页面：`支付设置`

如需接入 `GuuR 码支付` 这类标准 EPay 商户，可直接参考：

- [GuuR 码支付 EPay 配置示例](./payment-guur-epay-config.zh-CN.md)

### 4.1 字段映射

| 页面字段 | Option Key | 运行时变量/配置 | 说明 |
|---|---|---|---|
| 支付地址 | `PayAddress` | `operation_setting.PayAddress` | 易支付网关地址 |
| 易支付商户 ID | `EpayId` | `operation_setting.EpayId` | 商户号 |
| 易支付商户密钥 | `EpayKey` | `operation_setting.EpayKey` | 商户密钥 |
| 回调地址 | `CustomCallbackAddress` | `operation_setting.CustomCallbackAddress` | 留空时回退到 `ServerAddress` |
| 充值价格（x元/美金） | `Price` | `operation_setting.Price` | 充值金额换算价格 |
| 最低充值美元数量 | `MinTopUp` | `operation_setting.MinTopUp` | 最低充值量 |
| 充值分组倍率 | `TopupGroupRatio` | `common.UpdateTopupGroupRatioByJSONString` | 按用户分组调整金额 |
| 充值方式设置 | `PayMethods` | `operation_setting.PayMethods` | 用户充值页展示的 EPay 支付方式 |
| 自定义充值数量选项 | `payment_setting.amount_options` | `payment_setting.AmountOptions` | 用户可选充值档位 |
| 充值金额折扣配置 | `payment_setting.amount_discount` | `payment_setting.AmountDiscount` | 指定档位折扣 |

### 4.2 启用条件

用户侧 EPay 充值是否显示，取决于以下三个字段是否同时存在：

- `PayAddress`
- `EpayId`
- `EpayKey`

只要缺一个，`enable_online_topup` 就会是 `false`。

### 4.3 回调地址逻辑

- 如果填写了 `CustomCallbackAddress`，则回调基地址使用它
- 否则使用 `ServerAddress`

EPay 的回调地址是：

```text
{callback_base}/api/user/epay/notify
```

### 4.4 JSON 示例

#### 充值分组倍率 `TopupGroupRatio`

```json
{
  "default": 1,
  "vip": 0.9,
  "svip": 0.85
}
```

#### 充值方式设置 `PayMethods`

```json
[
  {
    "name": "支付宝",
    "color": "rgba(var(--semi-blue-5), 1)",
    "type": "alipay"
  },
  {
    "name": "微信",
    "color": "rgba(var(--semi-green-5), 1)",
    "type": "wxpay"
  },
  {
    "name": "银行卡",
    "color": "#111827",
    "type": "bank",
    "min_topup": "50"
  }
]
```

#### 自定义充值数量选项 `AmountOptions`

```json
[10, 20, 50, 100, 200, 500]
```

#### 充值金额折扣配置 `AmountDiscount`

```json
{
  "100": 0.95,
  "200": 0.9,
  "500": 0.85
}
```

### 4.5 标准 EPay 参数映射说明

标准 EPay 文档通常会要求这些参数：

- `pid`
- `type`
- `out_trade_no`
- `notify_url`
- `return_url`
- `name`
- `money`
- `sign`

在本项目里，管理员只需要填写页面上的平台配置字段，剩余参数由系统自动生成并提交。

## 5. Stripe 配置

对应页面：`Stripe 设置`

### 5.1 字段映射

| 页面字段 | Option Key | 运行时变量 | 说明 |
|---|---|---|---|
| API 密钥 | `StripeApiSecret` | `setting.StripeApiSecret` | Stripe Secret/RK |
| Webhook 签名密钥 | `StripeWebhookSecret` | `setting.StripeWebhookSecret` | `whsec_...` |
| 商品价格 ID | `StripePriceId` | `setting.StripePriceId` | 全局充值使用的 Price ID |
| 充值价格（x元/美金） | `StripeUnitPrice` | `setting.StripeUnitPrice` | 主要用于金额估算 |
| 最低充值美元数量 | `StripeMinTopUp` | `setting.StripeMinTopUp` | 最低充值量 |
| 允许输入促销码 | `StripePromotionCodesEnabled` | `setting.StripePromotionCodesEnabled` | Checkout 时允许输入优惠码 |

### 5.2 启用条件

用户侧 Stripe 充值是否显示，取决于以下三个字段是否同时存在：

- `StripeApiSecret`
- `StripeWebhookSecret`
- `StripePriceId`

### 5.3 Webhook 配置

推荐在 Stripe 后台配置：

```text
{ServerAddress}/api/stripe/webhook
```

需要包含的事件：

- `checkout.session.completed`
- `checkout.session.expired`

### 5.4 一个重要区别

普通充值和订阅套餐都能走 Stripe，但使用的字段不同：

- 普通充值：使用管理员页面里的全局 `StripePriceId`
- 订阅套餐：使用每个套餐自己的 `stripe_price_id`

也就是说：

- 管理员页里的 `StripePriceId` 不能替代套餐自己的 `stripe_price_id`
- 如果套餐没有单独配置 `stripe_price_id`，订阅支付会直接报错

### 5.5 关于 `StripeUnitPrice`

代码里普通充值创建 Stripe Checkout Session 时，实际使用的是：

- `StripePriceId`
- 购买数量 `Quantity = amount`

因此：

- 真正扣款以 Stripe Dashboard 中该 `PriceId` 对应的价格为准
- `StripeUnitPrice` 更像是前端金额估算、展示和校验辅助项

如果后台估算金额和 Stripe 实际价格不一致，优先检查 Stripe 平台中该 `PriceId` 的实际价格配置。

## 6. Creem 配置

对应页面：`Creem 设置`

### 6.1 字段映射

| 页面字段 | Option Key | 运行时变量 | 说明 |
|---|---|---|---|
| API 密钥 | `CreemApiKey` | `setting.CreemApiKey` | Creem API Key |
| Webhook 密钥 | `CreemWebhookSecret` | `setting.CreemWebhookSecret` | 用于 webhook 验签 |
| 测试模式 | `CreemTestMode` | `setting.CreemTestMode` | 使用测试 API 地址 |
| 产品配置表 | `CreemProducts` | `setting.CreemProducts` | JSON 数组，定义充值产品 |

### 6.2 启用条件

用户侧 Creem 充值是否显示，取决于以下条件：

- `CreemApiKey` 已配置
- `CreemProducts` 不是空数组 `[]`

注意：

- 用户侧入口显示不强依赖 `CreemWebhookSecret`
- 但正式环境建议一定要配置 `CreemWebhookSecret`
- 如果不开测试模式，某些订阅场景会强校验 webhook secret

### 6.3 产品配置说明

每条产品包含以下字段：

| 字段 | 说明 |
|---|---|
| `name` | 产品显示名称 |
| `productId` | Creem 产品 ID |
| `price` | 支付价格 |
| `quota` | 购买成功后增加的额度 |
| `currency` | 货币，当前页面支持 `USD` 或 `EUR` |

### 6.4 产品 JSON 示例

```json
[
  {
    "name": "基础充值包",
    "productId": "prod_basic_001",
    "price": 4.99,
    "quota": 500000,
    "currency": "USD"
  },
  {
    "name": "高级充值包",
    "productId": "prod_pro_001",
    "price": 19.99,
    "quota": 2500000,
    "currency": "USD"
  }
]
```

### 6.5 测试模式行为

测试模式打开后：

- Creem 请求会走测试环境接口
- 如果 webhook secret 为空，验签会被放宽

正式环境不要依赖这个行为。

## 7. Waffo 配置

对应页面：`Waffo 设置`

### 7.1 字段映射

| 页面字段 | Option Key | 运行时变量 | 说明 |
|---|---|---|---|
| 启用 Waffo | `WaffoEnabled` | `setting.WaffoEnabled` | 总开关 |
| 沙盒模式 | `WaffoSandbox` | `setting.WaffoSandbox` | 切换生产/沙盒 |
| API 密钥（生产） | `WaffoApiKey` | `setting.WaffoApiKey` | 生产环境 API Key |
| API 密钥（沙盒） | `WaffoSandboxApiKey` | `setting.WaffoSandboxApiKey` | 沙盒 API Key |
| RSA 私钥（生产） | `WaffoPrivateKey` | `setting.WaffoPrivateKey` | 生产私钥 |
| RSA 私钥（沙盒） | `WaffoSandboxPrivateKey` | `setting.WaffoSandboxPrivateKey` | 沙盒私钥 |
| Waffo 公钥（生产） | `WaffoPublicCert` | `setting.WaffoPublicCert` | 生产公钥证书 |
| Waffo 公钥（沙盒） | `WaffoSandboxPublicCert` | `setting.WaffoSandboxPublicCert` | 沙盒公钥证书 |
| 商户 ID | `WaffoMerchantId` | `setting.WaffoMerchantId` | 商户标识 |
| 货币 | `WaffoCurrency` | `setting.WaffoCurrency` | 页面中默认不可编辑 |
| 单价（USD） | `WaffoUnitPrice` | `setting.WaffoUnitPrice` | 每个充值单位折算价格 |
| 最低充值数量 | `WaffoMinTopUp` | `setting.WaffoMinTopUp` | 最低数量 |
| 回调通知地址 | `WaffoNotifyUrl` | `setting.WaffoNotifyUrl` | 留空则自动拼接 |
| 支付返回地址 | `WaffoReturnUrl` | `setting.WaffoReturnUrl` | 留空则自动拼接 |
| 支付方式列表 | `WaffoPayMethods` | `setting.GetWaffoPayMethods()` | 用户侧可选支付方式 |

### 7.2 启用条件

用户侧 Waffo 充值是否显示，取决于：

- `WaffoEnabled = true`
- 且当前环境下的密钥材料完整

生产环境要求：

- `WaffoApiKey`
- `WaffoPrivateKey`
- `WaffoPublicCert`

沙盒环境要求：

- `WaffoSandboxApiKey`
- `WaffoSandboxPrivateKey`
- `WaffoSandboxPublicCert`

### 7.3 默认回调与返回地址

如果页面不填写：

- `WaffoNotifyUrl` 默认是 `{callback_base}/api/waffo/webhook`
- `WaffoReturnUrl` 默认是 `{ServerAddress}/console/topup?show_history=true`

其中 `callback_base` 优先取 `CustomCallbackAddress`，否则取 `ServerAddress`。

### 7.4 支付方式配置

Waffo 的支付方式列表和 EPay 的 `PayMethods` 不是一套配置。

`WaffoPayMethods` 每项结构如下：

| 字段 | 说明 |
|---|---|
| `name` | 前端展示名 |
| `icon` | 图标地址或 base64 数据 |
| `payMethodType` | Waffo API 的支付方式类型 |
| `payMethodName` | Waffo API 的支付方式名称，可留空 |

### 7.5 WaffoPayMethods 示例

```json
[
  {
    "name": "Card",
    "icon": "/pay-card.png",
    "payMethodType": "CREDITCARD,DEBITCARD",
    "payMethodName": ""
  },
  {
    "name": "Apple Pay",
    "icon": "/pay-apple.png",
    "payMethodType": "APPLEPAY",
    "payMethodName": "APPLEPAY"
  },
  {
    "name": "Google Pay",
    "icon": "/pay-google.png",
    "payMethodType": "GOOGLEPAY",
    "payMethodName": "GOOGLEPAY"
  }
]
```

如果 `WaffoPayMethods` 为空或解析失败，系统会回退到默认的 `Card / Apple Pay / Google Pay`。

## 8. 用户侧支付入口的最终判断

用户充值页面最终会根据后端 `GetTopUpInfo` 的返回决定显示哪些支付方式。

| 返回字段 | 含义 |
|---|---|
| `enable_online_topup` | 是否启用 EPay |
| `enable_stripe_topup` | 是否启用 Stripe |
| `enable_creem_topup` | 是否启用 Creem |
| `enable_waffo_topup` | 是否启用 Waffo |
| `pay_methods` | EPay 主支付方式列表，必要时后端会自动附加 Stripe/Waffo 主入口 |
| `waffo_pay_methods` | Waffo 自定义支付方式列表 |
| `creem_products` | Creem 产品列表 |
| `amount_options` | 充值数量档位 |
| `discount` | 档位折扣 |

## 9. 订阅套餐支付和普通充值的区别

管理员支付页只解决“平台级支付通道”问题。

如果你还要启用“订阅套餐购买”，每个套餐还要单独配置支付商品 ID：

| 场景 | 需要的字段 |
|---|---|
| Stripe 订阅套餐 | 套餐自己的 `stripe_price_id` |
| Creem 订阅套餐 | 套餐自己的 `creem_product_id` |

这些字段在套餐编辑弹窗中配置，不在管理员支付页里统一配置。

## 10. 常见坑

### 10.1 密钥框是空的，不代表没保存

后端读取配置时会过滤 `Key / Secret / Token` 字段，避免敏感信息回显到前端。

### 10.2 不要只配 StripeUnitPrice 而不配 Stripe 后台 Price

Stripe 普通充值最终是按 `StripePriceId` 去创建 Checkout Session，实际收费以 Stripe 平台里的价格配置为准。

### 10.3 ServerAddress 必须是最终公网地址

如果这里填的是内网地址、容器名或错误域名，会导致：

- webhook 回调地址错误
- 支付成功后跳转地址错误
- 控制台内某些链接错误

### 10.4 反向代理场景建议单独设置 CustomCallbackAddress

如果外网回调域名和站点访问域名不同，建议：

- `ServerAddress` 填用户访问站点的正式域名
- `CustomCallbackAddress` 填第三方支付平台能访问到的回调域名

### 10.5 订阅支付报“未配置 StripePriceId / CreemProductId”

通常不是管理员支付页没配，而是对应套餐没配自己的商品 ID。

### 10.6 Waffo 的支付方式和 EPay 的支付方式不是同一套

- EPay 走 `PayMethods`
- Waffo 走 `WaffoPayMethods`

两边要分别配置。

## 11. 推荐配置顺序

建议按下面顺序配置：

1. 先填写 `ServerAddress`
2. 选择要启用的支付通道
3. 配置对应通道的密钥、价格和回调
4. 如果是 EPay，补 `PayMethods`、`AmountOptions`、`AmountDiscount`
5. 如果是 Creem，补产品列表
6. 如果是 Waffo，补支付方式列表
7. 如果要卖订阅套餐，再去套餐管理里填 `stripe_price_id / creem_product_id`
8. 最后用普通用户账号去充值页验证前端是否出现对应支付入口

## 12. 相关代码位置

- 页面入口：`web/src/pages/Setting/index.jsx`
- 支付聚合页：`web/src/components/settings/PaymentSetting.jsx`
- 通用设置：`web/src/pages/Setting/Payment/SettingsGeneralPayment.jsx`
- EPay 设置：`web/src/pages/Setting/Payment/SettingsPaymentGateway.jsx`
- Stripe 设置：`web/src/pages/Setting/Payment/SettingsPaymentGatewayStripe.jsx`
- Creem 设置：`web/src/pages/Setting/Payment/SettingsPaymentGatewayCreem.jsx`
- Waffo 设置：`web/src/pages/Setting/Payment/SettingsPaymentGatewayWaffo.jsx`
- Option API：`controller/option.go`
- Option 持久化与运行时同步：`model/option.go`
- 用户充值入口汇总：`controller/topup.go`
- Stripe 普通充值：`controller/topup_stripe.go`
- Creem 普通充值：`controller/topup_creem.go`
- Waffo 普通充值：`controller/topup_waffo.go`
- 套餐 Stripe 支付：`controller/subscription_payment_stripe.go`
- 套餐 Creem 支付：`controller/subscription_payment_creem.go`
