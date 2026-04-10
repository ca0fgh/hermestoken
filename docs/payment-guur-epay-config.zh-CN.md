# GuuR 码支付 EPay 配置示例

本文说明如何把 `GuuR 码支付` 的 EPay 接口配置到 `hermestoken` 的管理员支付页面。

参考文档：

- 页面跳转支付文档：<https://code.guur.cn/doc/epay_submit>

本文基于该文档中的参数定义：

- 请求地址：`POST /xpay/epay/submit.php`
- 核心参数：`pid`、`type`、`out_trade_no`、`notify_url`、`return_url`、`name`、`money`、`sign`

## 1. 适用页面

管理员后台：

```text
/console/setting?tab=payment
```

对应区块：

- `通用设置`
- `支付设置`

## 2. 先确认外部条件

在配置前，先准备好：

- 你的商户 `pid`
- 你的商户 `key`
- 对外可访问的站点域名，例如 `https://api.example.com`
- 确认第三方支付平台可以访问你的回调地址

如果你当前只是在本机用 `http://localhost:3000` 调试：

- 前端可以打开
- 但第三方支付平台通常无法回调到你的本机 `localhost`

所以正式接入时，`ServerAddress` 必须填公网地址。

## 3. 管理员页面怎么填

### 3.1 通用设置

| 页面字段 | 建议填写 |
|---|---|
| 服务器地址 | 你的公网域名，例如 `https://api.example.com` |

作用：

- 生成默认异步回调地址
- 生成默认支付完成跳转地址
- 作为用户侧控制台跳转基础地址

### 3.2 支付设置

| 页面字段 | 填写内容 | 对应 GuuR 参数/用途 |
|---|---|---|
| 支付地址 | `https://code.guur.cn/xpay/epay` | 基础网关地址 |
| 易支付商户ID | 你的 `pid` | 对应 `pid` |
| 易支付商户密钥 | 你的 `key` | 用于生成 `sign` |
| 回调地址 | 通常留空 | 留空则自动回退到 `ServerAddress` |
| 充值价格（x元/美金） | 比如 `7.3` | 业务换算，不是第三方字段原值 |
| 最低充值美元数量 | 比如 `1` | 业务限制 |
| 充值方式设置 | 配置 `alipay`、`wxpay` | 对应 `type` 可选值 |

## 4. 最重要的一个坑

`支付地址` 不要填成：

```text
https://code.guur.cn/xpay/epay/submit.php
```

应该填：

```text
https://code.guur.cn/xpay/epay
```

原因：

- 项目里使用的 EPay SDK 会自动在你填写的基地址后追加 `/submit.php`
- 如果你手动把 `/submit.php` 也写进去，最终请求地址可能会变成错误路径

## 5. 系统会自动生成什么

管理员页面不会让你手填所有 EPay 参数。你只需要填平台配置，项目会自动生成以下请求参数：

| GuuR 文档参数 | 由谁提供 |
|---|---|
| `pid` | 页面里的 `易支付商户ID` |
| `type` | 页面里的 `充值方式设置` + 用户前端选择 |
| `out_trade_no` | 系统自动生成 |
| `notify_url` | 系统自动生成 |
| `return_url` | 系统自动生成 |
| `name` | 系统自动生成 |
| `money` | 系统根据充值金额和价格规则自动计算 |
| `sign` | 系统用 `EpayKey` 自动生成 |
| `sign_type` | 系统固定使用 `MD5` |
| `device` | 系统默认传 `pc` |

## 6. 这个项目里各字段实际是怎么映射的

### 6.1 你填写的字段

| 项目配置字段 | 实际作用 |
|---|---|
| `PayAddress` | 作为 EPay SDK 的 `baseUrl` |
| `EpayId` | 映射到请求参数 `pid` |
| `EpayKey` | 用于请求签名 |
| `CustomCallbackAddress` | 如果填写，作为回调域名基础地址 |
| `Price` | 计算支付金额 |
| `MinTopUp` | 校验最低充值量 |
| `PayMethods` | 定义前端可选支付方式 |

### 6.2 系统自动拼出来的地址

如果 `回调地址` 留空：

- `notify_url = {ServerAddress}/api/user/epay/notify`
- `return_url = {ServerAddress}/console/log`

如果 `回调地址` 填了 `CustomCallbackAddress`：

- `notify_url = {CustomCallbackAddress}/api/user/epay/notify`
- `return_url` 仍然默认使用 `{ServerAddress}/console/log`

也就是说：

- 异步通知地址可以独立设置
- 前台跳转地址仍然看 `ServerAddress`

## 7. 推荐的最小可用配置

如果你只想先跑通支付宝和微信，可以直接这样填。

### 7.1 页面字段

| 页面字段 | 示例值 |
|---|---|
| 服务器地址 | `https://api.example.com` |
| 支付地址 | `https://code.guur.cn/xpay/epay` |
| 易支付商户ID | `1001` |
| 易支付商户密钥 | `你的商户key` |
| 回调地址 | 留空 |
| 充值价格（x元/美金） | `7.3` |
| 最低充值美元数量 | `1` |

### 7.2 充值方式设置

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
  }
]
```

## 8. 对照 GuuR 文档的理解

GuuR 文档里当前说明的页面跳转支付参数如下：

| 文档参数 | 文档含义 | 本项目里怎么来 |
|---|---|---|
| `pid` | 商户ID | `易支付商户ID` |
| `type` | 支付方式 | 用户选择的 `alipay` 或 `wxpay` |
| `out_trade_no` | 商户订单号 | 系统自动生成 |
| `notify_url` | 异步通知地址 | 系统自动生成 |
| `return_url` | 跳转通知地址 | 系统自动生成 |
| `name` | 商品名称 | 系统自动生成，类似 `TUC100` |
| `money` | 商品金额 | 系统自动计算 |
| `sitename` | 网站名称 | 当前项目未单独传 |
| `param` | 业务扩展参数 | 当前项目未单独传 |
| `clientip` | 用户IP | 当前项目未单独传 |
| `device` | 设备类型 | 项目默认传 `pc` |
| `channel_id` | 指定渠道ID | 当前项目未单独传 |
| `sign` | 签名字符串 | 系统自动生成 |
| `sign_type` | 签名类型 | 系统固定 `MD5` |

这意味着：

- 这家 GuuR 文档和项目当前 EPay 接法是兼容的
- 不需要改代码就能先跑通基础支付
- 如果你后面必须传 `channel_id`、`sitename`、`clientip` 等扩展字段，才需要改后端

## 9. 启用成功的判断标准

配置完成后，用户侧普通充值页会在以下条件满足时显示 EPay：

- `PayAddress` 不为空
- `EpayId` 不为空
- `EpayKey` 不为空

如果三者缺一个，页面会提示管理员未配置支付信息，或者用户侧不显示在线充值入口。

## 10. 配完以后怎么验证

建议按这个顺序验证：

1. 在管理员后台保存 `服务器地址`
2. 保存 `支付地址 / 商户ID / 商户密钥`
3. 保存 `充值方式设置`
4. 用普通用户账号打开充值页
5. 确认出现 `支付宝`、`微信` 两个选项
6. 发起一笔最小金额订单
7. 在 GuuR 后台确认已收到请求
8. 完成支付后确认系统收到异步通知

## 11. 常见错误

### 11.1 把支付地址填成了完整的 `submit.php`

表现：

- 无法拉起支付
- 请求地址不对

修正：

- 改成 `https://code.guur.cn/xpay/epay`

### 11.2 服务器地址填成了 `localhost`

表现：

- 支付平台回调失败
- 支付成功后浏览器跳转到错误地址

修正：

- 改成公网可访问地址

### 11.3 页面重新打开后密钥框是空的

表现：

- 误以为密钥丢了

原因：

- 后端不会把 `Key / Secret` 回显给前端

修正：

- 如果不想修改密钥，保持为空直接保存其他字段即可

### 11.4 只配置了支付通道，没有配置支付方式 JSON

表现：

- 用户前端看不到你想展示的支付方式

修正：

- 至少配置 `alipay`、`wxpay`

## 12. 相关代码位置

- 管理员页面：`web/src/pages/Setting/Payment/SettingsPaymentGateway.jsx`
- EPay 回调地址选择：`service/epay.go`
- EPay 下单：`controller/topup.go`
- EPay SDK：`github.com/Calcium-Ion/go-epay`

