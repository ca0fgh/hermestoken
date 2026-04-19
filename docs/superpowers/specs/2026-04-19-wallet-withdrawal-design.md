# Wallet Withdrawal Design

## Goal

为 `hermestoken` 新增一套面向普通用户的主余额提现能力，支持用户申请提现、管理员审批、管理员线下支付宝打款，以及完整的冻结余额与历史记录管理。

本次目标是：

- 允许用户从主余额 `quota` 发起提现申请
- 提现申请提交时要求填写支付宝账号
- 提现金额提交后立即冻结，避免用户重复消费
- 管理员可在线审批、驳回、确认已打款
- 管理员线下使用支付宝打款，系统只负责申请单与审批流，不对接自动打款
- 支持按金额区间配置多档手续费规则
- 支持在已打款状态下可选填写回执信息，但不强制要求
- 让用户和管理员都能分页查看提现记录

## Context

当前项目已经具备以下基础能力：

1. 用户余额体系
   - 用户主余额字段为 `user.quota`
   - 用户消费、充值、奖励发放都已围绕 `quota` 运行

2. 邀请返佣与奖励划转
   - 邀请返佣会先记入 `user.aff_quota`
   - 用户可通过现有 `aff_transfer` 能力将邀请奖励划转到主余额 `quota`
   - 当前“返佣到账 -> 划转到余额”已经打通

3. 支付与充值记录
   - 已有 `topup` 相关订单、状态与后台列表
   - 但 `topup` 语义是“充值入账”，不适合直接复用为提现申请单

4. 配置体系
   - 后台已有基于 `OptionMap` 的全局配置读写机制
   - 支付相关设置已有独立管理入口，适合承载提现配置

5. 后台与前台页面模式
   - 用户端已有 `钱包管理`
   - 管理端已有统一的分页表格、搜索、状态展示模式

## Confirmed Product Decisions

以下内容已与用户确认：

- 提现对象是用户主余额 `quota`
- 主余额中的全部额度都允许提现，不区分来源
- 第一版采用“管理员线下支付宝打款”，不做自动打款集成
- 用户申请提现时临时填写支付宝账号，不预绑定默认账号
- 用户提交提现申请后，系统立即冻结对应余额
- 最低提现金额由管理员配置
- 手续费支持按提现金额区间配置多档规则
- 手续费可为：
  - 固定金额
  - 比例金额
- 用户申请的是“提现金额”
- 实际打款金额 = 申请金额 - 手续费
- 提现申请、手续费展示、管理员打款金额统一使用货币金额口径，不使用 tokens 作为提现申请输入单位
- 若系统当前主界面采用 tokens 展示，提现区域仍需单独按当前货币金额展示
- 同一用户同一时间只允许存在 1 笔未完结提现单
  - `pending`
  - `approved`
- 提现状态分为：
  - `pending`
  - `approved`
  - `paid`
  - `rejected`
- `paid` 状态支持填写回执信息，但不是必填
- 提现入口放在 `钱包管理`
- 管理端需要独立的提现管理页面

## Out of Scope

本次不做：

- 支付宝自动转账或任何自动打款集成
- 银行卡、USDT、微信等其他提现通道
- 用户预绑定多个提现账号
- 风控评分、反洗钱规则、黑名单等高级风控
- 分账税务、发票或财务对账导出系统
- 批量打款
- 提现到账短信、邮件或站内通知增强

## Approaches Considered

### Approach A: 复用 `TopUp` 订单模型

做法：

- 在现有 `topup` 表上叠加“提现”语义
- 通过 `payment_method`、`status` 和附加字段区分充值与提现

问题：

- 充值与提现的业务方向完全相反
- `topup` 的状态机和字段名都偏向“入账”
- 后续维护时会让审批、冻结、打款回执语义变得混乱

### Approach B: 新增独立提现单模型 + 冻结余额字段（采用）

做法：

- 新增 `user_withdrawals` 表
- 为 `user` 增加冻结余额字段
- 提现申请、审批、打款全部围绕提现单状态机运行

优点：

- 充值与提现职责清楚
- 账务动作和审批状态一一对应
- 用户端与管理端都容易扩展
- 后续接入其他提现通道时仍可复用同一提现框架

代价：

- 需要增加一张新表与一个余额冻结字段
- 需要新增完整的用户端和管理员端接口

### Approach C: 不冻结余额，审批通过时再扣减

做法：

- 用户申请时只写申请单，不动余额
- 管理员审批通过时再从 `quota` 扣减

问题：

- 用户提交申请后仍可继续消费同一笔余额
- 管理员审核到打款之间会出现余额被挪用风险
- 线下流程与系统账务容易脱节

## Decision

采用 **Approach B**。

一句话定义本次能力：

- `用户从主余额提交提现申请，系统立即冻结对应金额，管理员在线审批并线下支付宝打款，最后以提现单状态和回执完成闭环`

## Information Architecture

### User Entry

提现入口放在：

- `钱包管理`

原因：

- 提现对象已经是主余额，不再是邀请返佣单独余额
- 用户对主余额的充值、消费、提现应收敛在同一页面内理解

### Admin Entry

管理端新增独立菜单：

- `提现管理`

职责：

- 查看全部提现申请
- 筛选待审核与待打款单
- 审批、驳回、确认已打款
- 查看用户余额与回执信息

## Data Model

### User Fields

在 `user` 上新增：

- `withdraw_frozen_quota`

语义：

- 表示已经提交提现申请、但尚未结单的冻结主余额

字段职责：

- `quota`
  - 当前可用主余额
- `withdraw_frozen_quota`
  - 已冻结、不可再消费的提现余额

补充要求：

- 所有涉及用户余额的后台展示与运营视图，都应同时展示 `quota` 与 `withdraw_frozen_quota`
- 现有管理员用户编辑弹窗至少需要增加冻结余额的只读展示，避免运营误把冻结中的金额当成可用余额

### Withdrawal Table

新增表：`user_withdrawals`

字段建议：

- `id`
- `user_id`
- `trade_no`
- `channel`
  - 第一版固定为 `alipay`
- `currency`
  - 第一版固定为当前提现使用的货币币种
- `exchange_rate_snapshot`
  - 申请时的汇率快照
- `available_quota_snapshot`
  - 用户提交申请前的可用余额快照
- `frozen_quota_snapshot`
  - 用户提交申请后的冻结余额快照
- `apply_amount`
  - 用户申请提现金额，面向管理员与财务查看
- `fee_amount`
  - 手续费金额
- `net_amount`
  - 实际应打款金额
- `apply_quota`
  - 对应冻结的内部额度
- `fee_quota`
  - 手续费对应的内部额度
- `net_quota`
  - 实际应出账额度
- `alipay_account`
- `alipay_real_name`
- `status`
  - `pending`
  - `approved`
  - `paid`
  - `rejected`
- `fee_rule_snapshot_json`
  - 命中的手续费规则快照
- `review_admin_id`
  - 审核通过的管理员
- `rejected_admin_id`
  - 驳回该单的管理员
- `paid_admin_id`
  - 确认已打款的管理员
- `review_note`
  - 审核通过备注
- `rejection_note`
  - 驳回原因
- `pay_receipt_no`
  - 可选，支付宝流水号/回执号
- `pay_receipt_url`
  - 可选，回执截图地址
- `paid_note`
  - 可选，管理员打款备注
- `reviewed_at`
- `paid_at`
- `created_at`
- `updated_at`

### Trade Number

每笔提现单生成唯一 `trade_no`，例如：

- `WDR202604190001`

要求：

- 全局唯一
- 便于用户和管理员检索

## Configuration Model

提现配置沿用现有 `OptionMap` 机制。

新增建议配置项：

- `WithdrawalEnabled`
- `WithdrawalMinAmount`
- `WithdrawalInstruction`
- `WithdrawalFeeRules`

### Withdrawal Fee Rules

`WithdrawalFeeRules` 存 JSON 数组，每条规则包含：

- `min_amount`
- `max_amount`
  - `0` 表示无上限
- `fee_type`
  - `fixed`
  - `ratio`
- `fee_value`
- `min_fee`
  - 比例手续费时可选
- `max_fee`
  - 比例手续费时可选
- `enabled`
- `sort_order`

规则说明：

- 按申请提现金额命中第一条有效区间规则
- 区间不允许重叠
- 若未命中任何规则，则手续费为 `0`

## Status Machine

提现单状态如下：

- `pending`
  - 用户已提交申请
  - 对应余额已冻结
  - 等待管理员审核
- `approved`
  - 管理员审核通过
  - 仍未线下打款
- `paid`
  - 管理员已完成线下支付宝打款
  - 支持补充回执，但不是必填
- `rejected`
  - 管理员已驳回
  - 对应冻结余额已退回用户

允许的状态流转：

- `pending -> approved`
- `pending -> rejected`
- `approved -> rejected`
- `approved -> paid`

不允许：

- `paid -> 其他状态`
- `rejected -> 其他状态`

## Accounting Model

### Submission

用户提交提现申请时：

- 锁定用户记录
- 在同一事务内校验当前用户不存在未完结提现单
- 校验主余额足够
- `quota -= apply_quota`
- `withdraw_frozen_quota += apply_quota`
- 创建 `user_withdrawals` 记录

结果：

- 用户可用余额立即减少
- 同时产生一笔待审核提现单

### Approve

管理员审核通过时：

- 只更新提现单状态为 `approved`
- 不再改动任何余额字段

原因：

- 资金已经在提交申请时冻结
- 审核通过只是进入“待线下打款”阶段

### Reject

管理员驳回时：

- 锁定提现单和用户记录
- `quota += apply_quota`
- `withdraw_frozen_quota -= apply_quota`
- 状态改为 `rejected`

结果：

- 冻结余额全部退回可用余额

### Mark Paid

管理员确认已打款时：

- 锁定提现单和用户记录
- `withdraw_frozen_quota -= apply_quota`
- 状态改为 `paid`
- 可选保存回执号、回执地址、打款备注

结果：

- 该笔冻结余额正式出账

## User Experience

### Wallet Summary

在 `钱包管理` 新增提现摘要区，至少展示：

- 当前余额
- 冻结中余额
- 可提现余额

当前确认规则下：

- 可提现余额 = 当前可用余额 `quota`

### Withdrawal Modal

用户点击“申请提现”后打开弹窗。

字段：

- 提现金额
- 支付宝账号
- 支付宝姓名
  - 必填，用于管理员核对

动态展示：

- 命中的手续费规则
- 手续费
- 实际到账金额
- 提现币种
- 提现说明

交互规则：

- 若存在未完结提现单，则按钮不可继续提交
- 若金额小于最低提现金额，直接提示
- 若金额大于当前可提现余额，直接提示

### Withdrawal History

用户提现记录列表支持分页，展示：

- 提现单号
- 申请金额
- 手续费
- 实际到账金额
- 支付宝账号
  - 默认掩码显示
- 状态
- 申请时间
- 审核时间
- 打款时间

详情中展示：

- 管理员备注
- 驳回原因
- 回执号
- 回执图片或链接
- 打款备注

## Admin Experience

### Withdrawal Management List

管理端列表支持分页与筛选：

- 状态
- 用户 ID
- 用户名
- 提现单号
- 支付宝账号
- 时间范围

建议默认排序：

- 按 `created_at desc`
- `pending` 与 `approved` 仍可通过快捷筛选优先处理

### Detail View

详情页或侧栏展示：

- 用户基本信息
- 余额快照
- 提现金额
- 手续费
- 实际打款金额
- 支付宝账号
- 支付宝姓名
- 申请时间
- 当前状态
- 审批历史字段

### Admin Actions

管理员操作分三类：

- `审核通过`
  - 仅用于 `pending`
  - 可填写审核备注
- `驳回`
  - 可用于 `pending`
  - 也可用于 `approved`
  - 驳回原因必填
- `确认已打款`
  - 仅用于 `approved`
  - 回执信息可选填写

## API Design

### User APIs

- `GET /api/user/withdrawal/config`
  - 返回提现开关、最低提现金额、提现说明、手续费规则摘要、可提现余额、冻结余额、是否存在未完结提现单、提现币种、提现展示口径

- `POST /api/user/withdrawals`
  - 提交提现申请
  - 请求体：
    - `amount`
      - 单位为提现币种金额，不是内部 quota 值
    - `alipay_account`
    - `alipay_real_name`

- `GET /api/user/withdrawals`
  - 获取我的提现记录列表
  - 支持分页

- `GET /api/user/withdrawals/:id`
  - 获取我的提现单详情

### Admin APIs

- `GET /api/admin/withdrawals`
  - 获取提现单列表
  - 支持分页与筛选

- `GET /api/admin/withdrawals/:id`
  - 获取提现单详情

- `POST /api/admin/withdrawals/:id/approve`
  - `pending -> approved`
  - 请求体可带：
    - `review_note`

- `POST /api/admin/withdrawals/:id/reject`
  - `pending/approved -> rejected`
  - 请求体：
    - `rejection_note`
  - 说明：
    - 驳回原因必填

- `POST /api/admin/withdrawals/:id/mark-paid`
  - `approved -> paid`
  - 请求体可带：
    - `pay_receipt_no`
    - `pay_receipt_url`
    - `paid_note`

## Validation Rules

### User Submission

用户提交提现申请时必须满足：

- 提现功能已开启
- 用户不存在未完结提现单
  - `pending`
  - `approved`
- 申请金额大于等于最低提现金额
- 申请金额小于等于当前可用主余额
- 申请金额必须按提现币种金额提交，不允许以前端 tokens 展示值直接作为申请金额
- 申请金额命中的手续费规则合法
- 实际到账金额必须大于 `0`
- `alipay_account` 必填
- `alipay_account` 需要去首尾空格
- `alipay_real_name` 必填，且长度要受控

### Fee Rules

管理员配置手续费规则时：

- 区间不允许重叠
- 区间必须可排序
- 固定手续费不得为负数
- 比例手续费不得为负数
- 比例手续费上限建议不超过 `100%`
- `min_fee` 与 `max_fee` 只在比例模式下生效
- `max_amount = 0` 表示无上限

### Status Operations

管理员状态流转时：

- `approve` 只能处理 `pending`
- `reject` 只能处理 `pending` 或 `approved`
- `mark-paid` 只能处理 `approved`
- 所有状态流转必须在事务中完成
- 所有写操作都应带行锁，避免并发修改

## Logging and Audit

以下动作都应写系统日志：

- 用户提交提现申请
- 管理员审核通过
- 管理员驳回
- 管理员确认打款

日志内容至少包含：

- 用户 ID
- 提现单号
- 金额
- 当前状态
- 管理员 ID

## Testing Strategy

### Unit Tests

- 手续费规则命中与金额计算
- 状态流转校验
- 金额快照与汇率快照写入
- 只允许 1 笔未完结单的判断逻辑

### Integration Tests

- 提现申请创建时正确冻结余额
- 提现申请在同一事务内阻止并发创建第二笔未完结单
- 驳回时正确退回余额
- 审批通过时不重复改余额
- 审批通过后再次驳回时能正确退回余额
- 确认打款时正确释放冻结余额
- 非法状态流转被拒绝

### Frontend Tests

- 钱包管理提现入口渲染
- 提现弹窗实时展示手续费和实际到账金额
- 提现记录分页与详情
- 管理端列表分页、筛选与状态操作按钮显示

## Rollout Plan

第一阶段：

- 后端模型
- 配置项
- 用户提现申请接口
- 钱包管理提现入口

第二阶段：

- 管理端提现管理页面
- 审批、驳回、确认打款接口
- 回执信息展示

第三阶段：

- 多语言
- 日志完善
- 测试数据与联调验证

## Open Questions

当前方案下没有阻塞实现的未决问题。

若后续业务变化，可再单独扩展：

- 是否允许多提现通道
- 是否区分“可提现余额”与“不可提现余额来源”
- 是否增加通知与财务导出
