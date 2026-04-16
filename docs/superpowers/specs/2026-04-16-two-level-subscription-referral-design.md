# Template-Based Referral Framework and Subscription Two-Level Referral Design

## Goal

为 `hermestoken` 设计一套可扩展的通用返佣框架，并以 `subscription_referral` 作为第一个落地返佣类型。

本次目标是：

- 用统一的 `返佣类型 + 分组 + 模板` 结构承载返佣规则
- 在 `subscription_referral` 下落地 `direct + team` 两级返佣
- 沿用现有分组体系，不再为不同返佣类型重新发明 group 维度
- 把项目里已经存在的“邀请人给被邀请人返佣”能力抽象进新框架
- 让模板、用户绑定、结算账本、历史快照语义一致

## Context

当前项目已经有一套可运行的订阅返佣实现，核心能力包括：

- 管理员按分组给邀请人开通订阅返佣总比例
- 邀请人按分组设置“给被邀请人的返佣比例”
- 邀请人可以对单个被邀请人做分组级覆盖
- 订单支付成功时，系统会先查单个被邀请人覆盖，没有再回退到邀请人的默认比例
- 结算结果会直接拆成两份：
  - 邀请人净得
  - 被邀请人得到

这说明“邀请人从自己应得返佣里切一部分给被邀请人”这条能力并不是新需求，而是当前订阅返佣里已经存在的能力。

本次设计不是把这部分能力删掉重做，而是要把它从“订阅返佣里的特例”提升成“通用返佣框架里的标准能力”。

同时，订阅返佣自身也不再只是一组零散 override，而是要进入统一模板模型：

- 模板决定用户在某个 `返佣类型 + 分组` 下的返佣身份
- 模板决定该类型和分组下的二级级别返佣规则
- 模板提供“给被邀请人返佣”的默认规则
- 用户只能调整自己给被邀请人的默认比例和单个被邀请人覆盖，不能改模板里的级别返佣规则

## Confirmed Product Decisions

以下规则已与用户确认：

- 返佣结算统一按 `返佣类型 + 分组` 区分
- 所有返佣类型都沿用现有分组体系
- 一个模板只对应一个 `返佣类型 + 分组`
- 同一个用户在同一个 `返佣类型 + 分组` 下，只能绑定一个生效模板
- 同一个用户在同一个返佣类型下，可以按不同分组绑定不同模板
- 模板同时承载三类信息：
  - 返佣身份
  - 二级级别返佣规则
  - 邀请人给被邀请人返佣规则
- 用户使用不同模板，可以在不同 `返佣类型 + 分组` 下拥有不同的：
  - 返佣身份
  - 返佣率
  - 被邀请人返佣率
- 用户不能修改模板里的二级级别返佣规则
- 用户只能修改自己“给被邀请人的返佣比例”：
  - 可改自己的默认比例
  - 可改单个被邀请人的覆盖比例
- “给被邀请人的返佣”只作用于付款用户的直接邀请人那一份返佣
- 更上层节点不能直接把自己的返佣分给付款用户
- 被邀请人返佣来自直接邀请人本单本来应得返佣，不是平台额外加发
- 被邀请人返佣比例范围为 `0% ~ 100%`
- 如果直接邀请人本单原始应得返佣为 `0`，则不生成被邀请人返佣明细
- 每个用户只能有一个直接邀请人，邀请关系是单链，不支持多个第一层邀请人
- `subscription_referral` 当前只允许两种模板身份：
  - `direct`
  - `team`
- `subscription_referral` 当前没有 `normal` 模板
- 在 `subscription_referral + group` 下没有绑定模板，等价于该用户不参与该类型和分组的返佣
- 在 `subscription_referral` 的祖先链里，`direct` 和 `team` 身份允许混排：
  - `direct -> team`
  - `team -> direct`
  - `team -> team`
  - `direct -> direct`
- 在订阅返佣链里，向上遍历时遇到“当前类型和分组下没有模板”的节点：
  - 跳过
  - 不断链
- 付款用户自己在当前 `subscription_referral + group` 下是否有模板、模板身份是什么，都不决定返佣入口和是否向上扩散
- 返佣链的级别参数只由“最近直接邀请人的活动模板”控制
- 更上层模板只决定自己是否具备参与资格和身份，不反向改写本单的返佣参数
- 模板的 `enabled` 只决定模板自己是否可被解析为活动模板
- 新旧引擎切换不由模板 `enabled` 决定，而由平台按 `referral_type + group` 单独控制
- 只有真实支付成功的订阅订单才进入 `subscription_referral` 结算
- 历史订单结算结果需要快照固化，后续改模板、改绑定、改邀请关系都不影响历史订单

## Scope Boundaries

本次方案包含：

- 通用返佣模板框架
- `subscription_referral` 的两级返佣规则
- 现有订阅“给被邀请人返佣”能力的抽象与复用
- 模板绑定、默认比例、单个被邀请人覆盖的统一模型

本次方案不包含：

- `project` 级别返佣
- 交易手续费返佣和提现返佣的具体结算公式
- 排行榜、经营分析大盘
- 一次性重写所有已有前端页面
- 自动跳过付款用户第一层普通邀请人继续向上找返佣节点

## Approaches Considered

### Approach A: 在当前订阅返佣实现上继续叠加

做法：

- 保留现有订阅返佣所有数据结构
- 继续叠加“模板”“多返佣类型”“模板身份”等新概念

问题：

- 当前实现是订阅专用，不是通用返佣框架
- 返佣身份、模板、分组、返佣类型会混在旧字段里
- 未来扩展到 `trade_fee_referral`、`withdraw_referral` 时会继续复制逻辑

### Approach B: 通用模板框架，订阅返佣先落地（采用）

做法：

- 统一抽象 `返佣类型 + 分组 + 模板 + 用户绑定`
- `subscription_referral` 先成为第一个落地返佣类型
- 复用当前已存在的“给被邀请人返佣”能力，但将其纳入统一模板和绑定结构

优点：

- 模板、身份、返佣类型、分组的语义统一
- 订阅返佣可以先落地，未来其他返佣类型直接复用框架
- 现有“给被邀请人返佣”功能不需要被废弃

代价：

- 需要把当前订阅返佣相关配置抽象成通用表结构
- 需要重新定义结算账本快照结构

## Decision

采用 **Approach B**。

一句话定义本次能力：

- `先建立通用返佣模板框架，再用它承载 subscription_referral 的 direct/team 两级返佣和现有 invitee rebate 能力`

## Framework Model

### Core Dimensions

所有返佣能力统一由以下维度决定：

- `referral_type`
  - 例如：
    - `subscription_referral`
    - `trade_fee_referral`
    - `withdraw_referral`
- `group`
  - 沿用现有分组体系
- `template`
  - 定义该 `referral_type + group` 下的返佣身份和返佣规则
- `binding`
  - 定义某个用户在该 `referral_type + group` 下使用哪个模板

### Template Model

一个模板只对应一个 `referral_type + group`。

通用模板层至少包含：

- `referral_type`
- `group`
- `name`
- `level_type`
- `enabled`
- `invitee_share_default_bps`

当前首版为了先落地 `subscription_referral`，模板表会额外直接承载这组“订阅两级规则字段”：

- `direct_cap_bps`
- `team_cap_bps`
- `team_decay_ratio`
- `team_max_depth`

模板职责：

- 定义用户在该 `referral_type + group` 下的返佣身份
- 定义该类型和分组下的返佣规则
- 提供“给被邀请人返佣”的默认比例

补充说明：

- `direct_cap_bps / team_cap_bps / team_decay_ratio / team_max_depth` 只对采用 `direct + team` 两级规则的返佣类型生效
- 当前只有 `subscription_referral` 使用这组字段
- `trade_fee_referral / withdraw_referral` 等后续返佣类型，不因为复用同一模板表就自动继承订阅两级规则；它们的具体字段和结算公式在各自方案里单独定义

对于采用 `direct + team` 两级规则的返佣类型（当前即 `subscription_referral`），模板参数还必须满足：

- `0 <= direct_cap_bps <= team_cap_bps <= 10000`
- `0 < team_decay_ratio <= 1`
- `team_max_depth >= 1`，且必须是正整数
- `0 <= invitee_share_default_bps <= 10000`

### User Binding Model

用户绑定模板的粒度是：

- `user_id + referral_type + group`

约束：

- 同一个用户在同一个 `referral_type + group` 下，只能有一个生效模板
- 同一个用户在同一个返佣类型下，可以按不同分组绑定不同模板

用户存在模板绑定记录后：

- 明确其在该类型和分组下的候选模板归属
- 只有当绑定模板 `enabled = true` 时
  - 该用户才在运行态获得该类型和分组下的活动返佣身份
  - 才使用该类型和分组下的级别返佣规则
  - 才以模板默认值参与“给被邀请人返佣”的默认比例解析

### Active Template Resolution

一笔订单进入结算时，必须先解析“活动模板”。

定义：

- 活动模板 = 付款用户第一层直接邀请人在当前 `referral_type + group` 下绑定、且模板 `enabled = true` 的模板

规则：

- 若第一层直接邀请人没有绑定模板
  - 本单不返佣
- 若第一层直接邀请人绑定了模板，但模板 `enabled = false`
  - 视为没有活动模板
  - 本单不返佣
- 若第一层直接邀请人绑定了活动模板
  - 该模板决定本单的返佣参数和入口身份

活动模板控制的参数包括：

- `level_type`
- `direct_cap_bps`
- `team_cap_bps`
- `team_decay_ratio`
- `team_max_depth`
- `invitee_share_default_bps`

这意味着：

- 一笔订单只会有一个活动模板
- 更上层祖先节点即使命中了自己的模板，也只提供：
  - 自己的参与资格
  - 自己的身份识别
- 更上层模板不会改写本单已经锁定的返佣参数

### Invitee Share Model

“给被邀请人返佣”是通用返佣框架里的附加层能力，不改变原返佣链和返佣池本身。

统一规则：

- 只作用于付款用户的直接邀请人
- 只作用于该直接邀请人本单本来应得的“最近一层返佣”
- 更上层节点不能直接把自己的返佣分给付款用户
- 比例范围为 `0 ~ 10000 bps`

有效比例优先级：

1. 单个被邀请人覆盖
2. 邀请人自己的默认比例
3. 模板默认比例

用户可修改的只有两层：

- 邀请人自己的默认比例
- 单个被邀请人覆盖比例

用户不可修改：

- 模板身份
- `direct_cap_bps`
- `team_cap_bps`
- `team_decay_ratio`
- `team_max_depth`

生效前提：

- 只有当直接邀请人在对应的 `referral_type + group` 下存在绑定，且绑定模板 `enabled = true` 时
  - 该直接邀请人的 invitee share 默认值或单个 invitee 覆盖才允许参与结算
- 若配置存在，但邀请人在该 `referral_type + group` 下没有绑定
  - 该配置不得在结算时被默默生效
  - 用户侧也不应把它展示为当前可用配置
- 若邀请人在该 `referral_type + group` 下虽然存在绑定，但绑定模板 `enabled = false`
  - 该配置也不得在结算时被默默生效
  - 用户侧不应把它展示为当前可用配置

## Subscription Referral Type

### Type-Specific Identity Set

`subscription_referral` 当前只允许两种模板身份：

- `direct`
- `team`

这意味着：

- 当前订阅返佣没有 `normal` 模板
- 在 `subscription_referral + group` 下没有模板，就等于该用户不参与该分组下的订阅返佣
- 付款用户自己是否有模板、是 `direct` 还是 `team`，都不会改变本单入口判断
- 祖先链中的 `direct` 和 `team` 可以混排存在

### Trigger Logic

订阅返佣的结算入口由：

- `referral_type = subscription_referral`
- `group = subscription_plan.upgrade_group`

共同决定。

当一笔订阅订单支付成功后：

1. 读取订单对应的 `upgrade_group`
2. 读取付款用户的直接邀请人
3. 在 `subscription_referral + group` 下解析这位直接邀请人的模板

结果分三种：

1. 直接邀请人不存在
   - 本单不返佣

2. 直接邀请人在该 `subscription_referral + group` 下没有模板
   - 本单不返佣
   - 不允许把“没有模板”当成第一层可跳过节点

3. 直接邀请人模板存在
   - 看模板身份：
     - `team` -> 进入 `team_direct`
     - `direct` -> 进入 `direct_with_team_chain`

### Settlement Modes

#### Mode 1: `team_direct`

条件：

- 付款用户的直接邀请人在 `subscription_referral + group` 下的模板身份是 `team`

结果：

- 不触发返佣链
- 只给这个最近 `team` 结算
- 更上层全部不参与
- 本单的 `team_cap_bps` 和默认 invitee share 都取自最近 `team` 的活动模板

#### Mode 2: `direct_with_team_chain`

条件：

- 付款用户的直接邀请人在 `subscription_referral + group` 下的模板身份是 `direct`

结果：

- 最近这个 `direct` 先拿直推返佣
- 再从这个 `direct` 的上级开始，继续往上解析 `subscription_referral + group` 模板
- 命中的 `team` 参与团队差额池
- 本单的 `direct_cap_bps / team_cap_bps / team_decay_ratio / team_max_depth / invitee_share_default_bps` 都取自最近 `direct` 的活动模板

### Traversal Rules

只有 `direct_with_team_chain` 模式才会向上遍历。

遍历规则：

1. 起点是最近 `direct` 的直接上级
2. 对每个祖先节点，都按当前 `subscription_referral + group` 解析模板
3. 若祖先节点没有模板：
   - 跳过
   - 不断链
4. 若祖先节点模板身份是 `direct`：
   - 不拿第二份返佣
   - 但不断链
5. 若祖先节点绑定了模板，但模板 `enabled = false`
   - 视为该祖先节点在当前 `referral_type + group` 下无模板
   - 跳过
   - 不断链
6. 若祖先节点模板身份是 `team`：
   - 记录为命中的团队返佣节点
7. 一直向上走到链路顶端

硬规则：

- 是否进入 `team_direct / direct_with_team_chain`，只由付款用户第一层直接邀请人的模板身份决定
- 付款用户自己在当前 `subscription_referral + group` 下是否有模板、模板身份是 `direct` 还是 `team`，都不改变返佣入口和是否向上扩散
- 第一层没有模板时直接拦截整单
- 上层没有模板只会失去资格，不会断链
- 更上层 `direct` 永远不拿第二份返佣
- `direct` 和 `team` 祖先节点允许混排；混排只会影响路径长度和团队命中结果，不改变“第一层决定入口”的原则

### Team Pool Logic

统一基数：

- `B = paid_amount × QuotaPerUnit`

两级返佣只拆成三种基础收益：

- `direct_reward_gross`
- `team_reward`
- `team_direct_reward_gross`

公式：

- `direct_reward_gross = floor(B × direct_cap_bps / 10000)`
- `team_pool = floor(B × (team_cap_bps - direct_cap_bps) / 10000)`
- `team_direct_reward_gross = floor(B × team_cap_bps / 10000)`

其中：

- `direct_reward_gross` 只属于最近 `direct`
- `team_pool` 只属于向上命中的 `team`
- `team_direct_reward_gross` 只属于最近 `team`

这里的参数来源统一是活动模板：

- `team_direct` 模式下：
  - 使用最近 `team` 的活动模板
- `direct_with_team_chain` 模式下：
  - 使用最近 `direct` 的活动模板

更上层 `team` 节点即使拥有自己的模板，也不会把自己的 `team_cap_bps / team_decay_ratio / team_max_depth` 带入本单

零金额规则：

- 任一 `reward_component` 只要最终 `reward_quota <= 0`
  - 就不生成该条结算明细
- 这条规则同样适用于：
  - `direct_reward`
  - `team_direct_reward`
  - `team_reward`
  - `invitee_reward`

### Path Distance Rule

`direct_with_team_chain` 模式下，团队池的递减按真实路径距离计算。

定义：

- 起点：最近直接邀请付款用户的那个 `direct`
- 距离：从这个 `direct` 往上走到某个命中 `team` 节点经过的真实边数

权重：

- `w_i = r^(d_i - 1)`

其中：

- `r = team_decay_ratio`
- `d_i = 第 i 个团队节点的真实路径距离`

过滤：

- 超过 `team_max_depth` 的团队节点不参与分配

若过滤后没有任何有效 `team` 节点：

- 不生成任何 `team_reward`
- `team_pool` 不回补给最近 `direct`
- `team_pool` 不回补给付款用户或 `invitee_reward`
- 这部分额度视为本单未分配返佣，不发放

分配：

- `share_i = w_i / Σw`
- `team_reward_i = floor(team_pool × share_i)`

尾差：

- 统一补给最近团队节点

### Invitee Share Layer

“给被邀请人返佣”发生在基础返佣计算之后。

它不改变：

- 是否触发返佣
- 是否触发返佣链
- 团队池大小
- 团队池分配

它只改变最近直接邀请人那一份“最近一层返佣”的最终落账结果。

#### Eligible Source Rewards

在 `subscription_referral` 下，当前允许被切分给被邀请人的源收益只有：

- `direct_reward_gross`
- `team_direct_reward_gross`

不允许被切分的收益：

- `team_reward`

#### Invitee Share Formula

设：

- `source_reward_gross = 最近直接邀请人本单原始应得返佣`
- `invitee_share_bps = 生效的被邀请人返佣比例`

则：

- `invitee_reward = floor(source_reward_gross × invitee_share_bps / 10000)`
- `immediate_inviter_reward_net = source_reward_gross - invitee_reward`

若：

- `source_reward_gross <= 0`
- 或 `invitee_reward <= 0`

则：

- 不生成 `invitee_reward`
- 不生成 `0` 金额流水

#### Existing Subscription Capability Reuse

当前项目已经存在两层订阅 invitee rebate 配置能力：

- 邀请人按分组设置默认比例
- 邀请人按“被邀请人 + 分组”设置覆盖比例

新框架应直接复用这两层能力的产品语义与优先级，只把维度扩展为：

- 默认比例：`inviter + referral_type + group`
- 覆盖比例：`inviter + invitee + referral_type + group`

对于 `subscription_referral`，这意味着：

- 当前已有的订阅“给被邀请人返佣”功能不是废弃，而是迁移进通用框架
- 现有前端交互可以先保留为 `subscription_referral` 的专用 facade

## Ledger Semantics

### Gross vs Net Rule

最近直接邀请人的“最近一层返佣”在 invitee share 生效后，必须区分：

- `gross`
  - 切分前原始应得返佣
- `net`
  - 切分后邀请人最终到账返佣

硬规则：

- `direct_reward`
  - `reward_quota` 记录净额
  - `gross_reward_quota_snapshot` 记录毛额
- `team_direct_reward`
  - `reward_quota` 记录净额
  - `gross_reward_quota_snapshot` 记录毛额
- `invitee_reward`
  - `reward_quota` 记录被邀请人拿到的切分额度
  - `gross_reward_quota_snapshot` 记录对应源返佣的毛额
- `team_reward`
  - 不参与 invitee share
  - 只有最终到账额，没有毛额/净额拆分语义

### Record Field Semantics

为了避免后端实现出两套不同账本语义，各 `reward_component` 的关键字段统一按以下口径落库：

- `direct_reward`
  - `reward_quota = direct_reward_net`
  - `gross_reward_quota_snapshot = direct_reward_gross`
  - `invitee_share_bps_snapshot = 生效 share 比例，没有 share 时记 0`
  - `applied_rate_bps = direct_cap_bps`
  - `source_reward_component = NULL`

- `team_direct_reward`
  - `reward_quota = team_direct_reward_net`
  - `gross_reward_quota_snapshot = team_direct_reward_gross`
  - `invitee_share_bps_snapshot = 生效 share 比例，没有 share 时记 0`
  - `applied_rate_bps = team_cap_bps`
  - `source_reward_component = NULL`

- `team_reward`
  - `reward_quota = 该团队节点最终到账额`
  - `gross_reward_quota_snapshot = NULL`
  - `invitee_share_bps_snapshot = NULL`
  - `applied_rate_bps = NULL`
  - `source_reward_component = NULL`

- `invitee_reward`
  - `reward_quota = 被邀请人实际到账额`
  - `gross_reward_quota_snapshot = 对应源返佣毛额`
  - `invitee_share_bps_snapshot = 生效 share 比例`
  - `applied_rate_bps = NULL`
  - `source_reward_component = direct_reward 或 team_direct_reward`

这意味着：

- 最近直接邀请人的账本真相是“净额”
- 审计毛额靠 `gross_reward_quota_snapshot`
- 被邀请人账本真相是“切出部分”
- `team_reward` 账本保持纯最终结果，不承载切分语义

## Worked Examples

假设：

- `B = 1000 quota`
- `direct_cap_bps = 1000`
- `team_cap_bps = 2500`
- `team_decay_ratio = 0.5`
- `team_max_depth = 10`

### Example 1: `direct1 -> user`

前提：

- `subscription_referral + group` 下，`direct1` 模板身份是 `direct`
- `invitee_share_bps = 0`

结果：

- `direct1 gross = 100`
- 不生成 `invitee_reward`
- `direct1 net = 100`
- 未命中任何有效 `team` 节点，因此 `team_pool` 不发放

### Example 2: `team1 -> direct1 -> user`

前提：

- `direct1` 模板身份是 `direct`
- `team1` 模板身份是 `team`
- `invitee_share_bps = 0`

结果：

- `direct1 gross = 100`
- `direct1 net = 100`
- `team1 = 150`

### Example 3: `team2 -> direct2 -> team1 -> direct1 -> user`

前提：

- `direct1` 模板身份是 `direct`
- `team1`、`team2` 模板身份是 `team`
- `direct2` 有模板但身份是 `direct`
- `invitee_share_bps = 0`

结果：

- `direct1 gross = 100`
- `direct1 net = 100`
- `team_pool = 150`
- `team1` 距离 `1`
- `team2` 距离 `3`
- 若 `r = 0.5`
  - `team1 = 120`
  - `team2 = 30`

### Example 4: `team1 -> user`

前提：

- `team1` 模板身份是 `team`
- `invitee_share_bps = 0`

结果：

- `team1 gross = 250`
- `team1 net = 250`
- 不触发返佣链

### Example 5: `team1 -> direct1 -> user` 且 `invitee_share_bps = 3000`

结果：

- `direct1 gross = 100`
- `user invitee_reward = 30`
- `direct1 net = 70`
- `team1 = 150`

### Example 6: `team1 -> user` 且 `invitee_share_bps = 4000`

结果：

- `team1 gross = 250`
- `user invitee_reward = 100`
- `team1 net = 150`

## Data Model

### 1. Template Table

新增表：`referral_templates`

建议字段：

- `id`
- `referral_type`
- `group`
- `name`
- `level_type`
- `enabled`
- `direct_cap_bps`
- `team_cap_bps`
- `team_decay_ratio`
- `team_max_depth`
- `invitee_share_default_bps`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

约束：

- `referral_type + group + name` 唯一
- `invitee_share_default_bps` 范围为 `0 ~ 10000`
- 对 `subscription_referral` 来说：
  - `level_type` 只能是 `direct / team`
  - `0 <= direct_cap_bps <= team_cap_bps <= 10000`
  - `0 < team_decay_ratio <= 1`
  - `team_max_depth >= 1`，且必须是正整数

说明：

- 当前首版把 `subscription_referral` 的两级规则字段直接放在 `referral_templates` 表里
- 这些字段只在 `subscription_referral` 下参与解析与校验
- 对其他返佣类型，若后续不采用 `direct + team` 两级规则，则不应强行套用这些字段语义

### 2. Binding Table

新增表：`referral_template_bindings`

建议字段：

- `id`
- `user_id`
- `referral_type`
- `group`
- `template_id`
- `invitee_share_override_bps`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

约束：

- `user_id + referral_type + group` 唯一
- 同一个用户在同一个 `referral_type + group` 下只能有一个生效模板
- `binding.referral_type` 必须等于 `template.referral_type`
- `binding.group` 必须等于 `template.group`
- `invitee_share_override_bps` 若不为空，范围必须为 `0 ~ 10000`

说明：

- `invitee_share_override_bps`
  - 是用户自己覆盖模板默认 invitee share 的默认值
  - 只影响“给被邀请人返佣”
  - 不影响模板里的级别返佣规则

### 3. Invitee Override Table

新增表：`referral_invitee_share_overrides`

建议字段：

- `id`
- `inviter_user_id`
- `invitee_user_id`
- `referral_type`
- `group`
- `invitee_share_bps`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

约束：

- `inviter_user_id + invitee_user_id + referral_type + group` 唯一
- `invitee_share_bps` 范围必须为 `0 ~ 10000`
- 必须校验：
  - `invitee.inviter_id == inviter_user_id`
  - `inviter_user_id` 在对应的 `referral_type + group` 下存在绑定

说明：

- 这是现有 `subscription_referral_invitee_overrides` 的通用化版本

### 4. Settlement Batch Table

新增表：`referral_settlement_batches`

建议字段：

- `id`
- `referral_type`
- `group`
- `source_type`
- `source_id`
- `source_trade_no`
- `payer_user_id`
- `immediate_inviter_user_id`
- `active_template_snapshot_json`
- `team_chain_snapshot_json`
- `settlement_mode`
- `quota_per_unit_snapshot`
- `status`
- `settled_at`
- `created_at`
- `updated_at`

说明：

- 对订阅返佣来说：
  - `source_type = subscription_order`
  - `group = subscription_plan.upgrade_group`
- `immediate_inviter_user_id`
  - 保存付款用户第一层直接邀请人的用户 ID
  - 在 `team_direct` 模式下它可能是 `team`
  - 在 `direct_with_team_chain` 模式下它可能是 `direct`
- `active_template_snapshot_json`
  - 保存最近直接邀请人的活动模板快照
  - 它是本单级别返佣参数和“模板层默认 invitee share”的唯一模板真相
  - 邀请人自己的默认比例或单个 invitee 覆盖不写入这里
  - 它们只通过结算明细中的 `invitee_share_bps_snapshot` 体现最终生效结果
- `team_chain_snapshot_json`
  - `team_direct` 模式下记 `NULL`
  - `direct_with_team_chain` 模式下至少记录：
    - 命中团队节点为空时，也要记录空数组/空列表，而不是 `NULL`
    - 命中团队节点的有序列表
    - 每个团队节点的 `user_id`
    - `path_distance`
    - `matched_team_index`
    - `weight_snapshot`
    - `share_snapshot`
  - 不再单独保留第二份泛化模板快照字段
  - 避免与活动模板快照职责重复

### 5. Settlement Record Table

新增表：`referral_settlement_records`

建议字段：

- `id`
- `batch_id`
- `referral_type`
- `group`
- `beneficiary_user_id`
- `beneficiary_level_type`
- `reward_component`
- `source_reward_component`
- `path_distance`
- `matched_team_index`
- `weight_snapshot`
- `share_snapshot`
- `gross_reward_quota_snapshot`
- `invitee_share_bps_snapshot`
- `pool_rate_bps_snapshot`
- `applied_rate_bps`
- `reward_quota`
- `reversed_quota`
- `debt_quota`
- `status`
- `created_at`
- `updated_at`

对 `subscription_referral`，首批 `reward_component` 包括：

- `direct_reward`
- `team_reward`
- `team_direct_reward`
- `invitee_reward`

字段语义：

- `beneficiary_level_type`
  - `direct_reward` 记录最近直接邀请人的模板身份 `direct`
  - `team_direct_reward` 记录最近直接邀请人的模板身份 `team`
  - `team_reward` 记录命中团队节点的模板身份 `team`
  - `invitee_reward`
    - 若付款用户在当前 `referral_type + group` 下存在绑定，且绑定模板 `enabled = true`，则记录付款用户当时的模板身份
    - 若付款用户在当前 `referral_type + group` 下无模板，或绑定模板 `enabled = false`，则记 `NULL`
- `gross_reward_quota_snapshot`
  - 最近直接邀请人被切分前的原始应得返佣
- `invitee_share_bps_snapshot`
  - 最近直接邀请人本单生效的 invitee share 比例
- `path_distance`
  - 仅 `team_reward` 使用
  - 记录该团队节点相对最近 `direct` 的真实路径距离
- `matched_team_index`
  - 仅 `team_reward` 使用
  - 记录命中团队节点按距离从近到远排序后的序号
- `weight_snapshot`
  - 仅 `team_reward` 使用
  - 记录该团队节点参与团队池计算时的原始权重
- `share_snapshot`
  - 仅 `team_reward` 使用
  - 记录该团队节点最终分到的团队池份额占比
- `pool_rate_bps_snapshot`
  - 仅 `team_reward` 使用
  - 记录本单团队池对应的比例，即 `team_cap_bps - direct_cap_bps`
  - 对 `direct_reward / team_direct_reward / invitee_reward` 记 `NULL`
- `reward_quota`
  - 该条明细最终实际到账额度
- `source_reward_component`
  - 对 `invitee_reward` 来说，用来标记它来自：
    - `direct_reward`
    - 或 `team_direct_reward`

### 6. Engine Route Table

新增表：`referral_engine_routes`

建议字段：

- `id`
- `referral_type`
- `group`
- `engine_mode`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

其中：

- `engine_mode`
  - `legacy`
  - `template`

约束：

- `referral_type + group` 唯一

说明：

- 新旧引擎切换由这张表控制
- 模板本身的 `enabled` 只决定模板是否可被解析为活动模板
- 两者职责不同，不能混用

## Admin and User Surfaces

### Admin Surfaces

后台至少需要三块能力：

1. 模板管理
- 创建模板
- 编辑模板
- 指定模板的 `referral_type + group + level_type`
- 指定级别返佣规则
- 指定模板默认 invitee share

2. 用户模板绑定
- 给用户在某个 `referral_type + group` 下分配模板

3. 邀请关系管理
- 维护用户直接邀请人
- 保证每个用户只能有一个直接邀请人
- 校验不能自邀
- 校验不能形成环

### User Surfaces

用户侧至少需要两块能力：

1. 查看自己在命中的 `referral_type + group` 下的返佣配置摘要
2. 修改自己给被邀请人的返佣比例：
- 修改默认比例
- 修改单个被邀请人覆盖

补充规则：

- 只有当用户自己在对应的 `referral_type + group` 下存在绑定，且绑定模板 `enabled = true` 时
  - 才允许在用户侧修改默认 invitee share 或单个 invitee 覆盖

用户侧不能修改：

- 模板身份
- 级别返佣率
- 团队递减规则
- 最大深度

## Rollout and Migration

### Engine Boundary

新模板框架与当前旧版订阅返佣引擎并存，但建议优先把现有订阅 invitee rebate 能力迁入新框架。

建议策略：

- 若 `referral_engine_routes.engine_mode = template`
  - 只执行新框架结算
- 若 `referral_engine_routes.engine_mode = legacy`
  - 继续保持旧订阅返佣逻辑
- 若某个 `referral_type + group` 没有配置 `referral_engine_routes`
  - 默认按 `legacy` 处理
  - 不允许因为缺少路由配置而进入不确定状态

目标：

- 避免同一笔订单被两套返佣逻辑重复结算
- 允许按 `referral_type + group` 灰度切换

补充规则：

- 若某个用户绑定到了 `enabled = false` 的模板
  - 该模板不会被解析为活动模板
  - 在新框架里等价于“该用户在该 `referral_type + group` 下无活动模板”
- 这不会自动把单个用户回退到旧引擎
  - 订单是否走旧引擎，只看 `referral_engine_routes`

### Existing Capability Reuse

现有订阅实现里的以下能力应被视为迁移来源，而不是废弃物：

- 邀请人按分组设置默认 invitee 比例
- 邀请人按被邀请人 + 分组设置覆盖比例
- 结算时“单独覆盖优先，默认分组比例次之”的优先级

迁移方向：

- `subscription_referral_overrides`
  -> `subscription_referral` 下的模板候选参数与待绑定用户清单的种子数据来源
- `UserSetting.SubscriptionReferralInviteeRateBpsByGroup`
  -> `referral_template_bindings.invitee_share_override_bps`
- `subscription_referral_invitee_overrides`
  -> `referral_invitee_share_overrides`

语义说明：

- 旧 `subscription_referral_overrides` 只包含“邀请人 + 分组 + 总返佣率”语义
- 它不能自动推出：
  - `team` 身份
  - 团队链参数
  - 团队模板
- 它也不能自动决定：
  - 用户最终应绑定哪个模板
  - 是否应该复用共享模板，还是拆成多套模板
- 因此它在迁移时只能作为模板候选参数和待绑定用户清单的种子数据来源，而不能独立完成模板落地
- 当前订阅实现里的 invitee 比例，迁入新框架后应解释为“切最近直接邀请人即时返佣的比例”
- 不再继续按旧实现中的“总返佣 bps 拆分”去理解新模板框架里的 invitee share

推荐迁移方式：

1. 先按业务决定哪些用户在 `subscription_referral + group` 下应当绑定 `direct` 模板、哪些应当绑定 `team` 模板
2. 对于原有 `subscription_referral_overrides` 中存在的数据：
   - 若目标是 `direct` 模板
     - 可用其 `total_rate_bps` 作为 `direct_cap_bps` 的种子来源
   - 若目标是 `team` 模板
     - 需要平台显式指定 `team_cap_bps / team_decay_ratio / team_max_depth`
     - 不能从旧 override 自动推断
3. 再迁移用户默认 invitee share 和单个 invitee override

迁移前置条件：

- 只有当某个用户在对应的 `subscription_referral + group` 下已经绑定模板，且绑定模板 `enabled = true`
  - 该用户原有的 invitee default / invitee override 配置才可迁入新框架并立即生效
- 若旧配置存在，但对应用户尚未绑定模板
  - 该配置进入“待迁移”清单
  - 不应在模板引擎下被默默生效
- 若旧配置存在，且对应用户虽然已经绑定模板，但绑定模板 `enabled = false`
  - 该配置可以迁入为保留数据
  - 但必须保持为未生效状态，不能在模板引擎下被默默生效

切换要求：

- 在某个 `subscription_referral + group` 切到 `template` 模式前
  - 需要完成该分组内目标用户的模板绑定
  - 需要完成旧 invitee 配置的迁移或显式放弃
- 否则不应切换该分组的引擎路由

### Existing UI Boundary

当前“邀请返佣”页面可以先继续作为 `subscription_referral` 的专用用户界面存在，只是底层数据来源改为通用模板框架。

这意味着：

- 第一阶段不必重写整套用户页面
- 但页面文案和接口语义应逐步从“订阅专用 override”升级为“当前返佣类型和分组下的 invitee share 配置”

## Acceptance Criteria

满足以下条件即可认为本次方案成立：

1. 平台可以创建按 `referral_type + group` 区分的返佣模板
2. 平台可以把模板绑定给用户，绑定粒度是 `user + referral_type + group`
3. 模板绑定必须和模板自己的 `referral_type + group` 一致，不能跨维度脏绑
4. `subscription_referral` 下模板身份只允许 `direct / team`
5. 在 `subscription_referral + group` 下：
   - 第一层直接邀请人没有模板时，整笔不返佣
   - 第一层直接邀请人虽然绑定了模板，但模板 `enabled = false` 时，等价于没有活动模板，整笔不返佣
   - 第一层是 `team` 模板时，只结算 `team_direct`
   - 第一层是 `direct` 模板时，触发团队返佣链
6. 向上遍历时，未绑定当前 `referral_type + group` 模板的祖先节点，或绑定模板但模板 `enabled = false` 的祖先节点，都会被跳过，但不会断链
7. 一笔订单的 `direct_cap_bps / team_cap_bps / team_decay_ratio / team_max_depth / invitee_share_default_bps` 只由最近直接邀请人的活动模板决定
8. `direct_reward` 和 `team_direct_reward` 的账本记录净额，毛额通过 `gross_reward_quota_snapshot` 审计
9. `invitee_reward` 明确记录被邀请人拿到的切分额度，并标明来源是 `direct_reward` 或 `team_direct_reward`
10. 模板 `enabled` 只控制模板是否可被解析为活动模板；新旧引擎切换由 `referral_engine_routes` 控制
11. 未配置 `referral_engine_routes` 的 `referral_type + group` 默认走 `legacy`
12. 旧 `subscription_referral_overrides` 可以作为模板候选参数和待绑定用户清单的迁移种子，但不能自动推出 team 模板，也不能自动决定最终模板绑定
13. 旧 invitee 配置只有在对应用户完成模板绑定，且绑定模板 `enabled = true` 后才能迁入并立即生效
14. 最近直接邀请人可按默认比例或单个被邀请人覆盖，把自己本单即时返佣切一部分给付款用户
    - 生效优先级必须是：单个被邀请人覆盖 > 邀请人自己的默认比例 > 模板默认比例
15. `invitee_reward` 只来自最近直接邀请人的即时返佣，不来自更上层 `team_reward`
16. 用户只能修改 invitee share，不能修改模板里的级别返佣规则
17. 现有订阅 invitee rebate 配置可以迁移或兼容到新模板框架
18. 历史订单在修改模板、绑定、邀请关系后仍保持原结算结果不变
19. 对 `subscription_referral` 来说，模板参数必须满足 `0 <= direct_cap_bps <= team_cap_bps <= 10000`、`0 < team_decay_ratio <= 1`、`team_max_depth >= 1`
    - 对其他返佣类型，若未采用 `direct + team` 两级规则，则不强行套用这组字段的解析、校验与结算语义
20. `direct_with_team_chain` 模式下若最终没有命中任何有效 `team` 节点，则不生成 `team_reward`，且 `team_pool` 不改发给其他角色
21. 每个用户只能有一个直接邀请人，邀请关系是单链，不支持多个第一层邀请人
22. 付款用户自己在当前 `subscription_referral + group` 下是否有模板、模板身份是什么，都不改变返佣入口和是否向上扩散；只看第一层直接邀请人的模板身份
23. `subscription_referral` 的祖先链允许 `direct / team` 混排，混排会影响路径距离与团队命中，但不改变入口判定规则
24. `invitee_share_override_bps` 和 `invitee_share_bps` 的取值范围都必须是 `0 ~ 10000`
25. `invitee_reward` 的 `beneficiary_level_type` 必须按付款用户在当前 `referral_type + group` 下已启用模板的实际身份落账；无模板或模板 `enabled = false` 时记 `NULL`
26. invitee share 默认值和单个 invitee 覆盖只有在邀请人自己已绑定对应 `referral_type + group` 模板，且模板 `enabled = true` 时才允许生效
27. 任一 `reward_component` 只要最终 `reward_quota <= 0`，就不生成该条结算明细
28. `referral_invitee_share_overrides` 写入时必须校验邀请人在对应 `referral_type + group` 下已存在绑定
29. `direct_with_team_chain` 模式下，`team_chain_snapshot_json` 即使没有命中任何有效团队节点，也必须记录空链快照而不是 `NULL`
