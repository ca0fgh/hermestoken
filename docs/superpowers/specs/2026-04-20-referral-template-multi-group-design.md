# Multi-Group Referral Template Management Design

## Goal

为返佣模板后台增加“一个模板配置覆盖多个分组”的管理能力，同时保持现有返佣结算引擎的稳定性。

本次目标是：

- 让管理员在后台一次配置一套返佣模板参数，并选择多个系统分组
- 保持运行时仍按单个 `referral_type + group` 命中模板，不重写现有结算引擎
- 把“一个模板覆盖多个分组”的体验实现为“一个模板组管理多个单分组模板行”
- 兼容当前已存在的单分组模板、用户绑定、单个被邀请人覆盖比例、历史结算记录

## Context

当前返佣模板体系已经上线了按 `referral_type + group` 维度建模的能力：

- 模板表 `referral_templates` 当前只有单个 `group` 字段
- 绑定表 `referral_template_bindings` 也是按 `user_id + referral_type + group` 唯一
- 运行时结算入口按单个 `group` 查找活动模板绑定
- 现有后台模板管理页的“分组”字段是单选

这意味着：

- 当前系统已经支持“一个返佣类型下，不同分组使用不同模板”
- 但不支持“管理员以一条管理记录同时维护多个分组”

用户本次要的不是重做结算规则，而是提升后台配置体验：

- 希望“返佣模板”支持多个分组
- 不希望为同一套返佣参数手工重复创建多条模板

## Confirmed Product Decisions

以下规则已与用户确认：

- 管理端要支持一个返佣模板配置选择多个分组
- 最优方案不是把运行时模板改成真正的 `groups[]`
- 最优方案是前台多选分组，后端仍落成多条单分组模板记录
- 为了让这些单分组模板在后台仍表现为“同一个模板”，需要增加轻量模板组标识
- 用户绑定、invitee override、结算命中、历史账本继续按单个 `group` 工作
- 第一阶段不重构返佣结算引擎，只改模板管理面与模板管理接口
- 出现分组冲突时，保存应失败并提示具体冲突分组，不做静默覆盖

## Scope Boundaries

本次方案包含：

- 返佣模板管理页支持多选分组
- 模板组聚合展示和批量保存
- 模板表增加用于聚合管理的轻量字段
- 模板接口支持聚合读写
- 兼容旧单分组模板数据

本次方案不包含：

- 重写 `subscription_referral` 结算引擎
- 把绑定模型改成一个用户绑定一个 `groups[]`
- 把 invitee override 改成多分组数组模型
- 修改历史结算记录结构
- 一次性抽象所有未来返佣类型的多分组编辑器

## Approaches Considered

### Approach A: 真正把模板模型改成 `groups[]`

做法：

- `referral_templates` 从单个 `group` 改成数组字段
- 绑定、覆盖、结算、查询全部改为支持多分组模板

优点：

- 数据模型表面上更“自然”
- 单条记录真的是单个多分组模板

问题：

- 现有结算和绑定逻辑全部建立在单 `group` 事实上
- 需要同步改模板解析、绑定唯一性、覆盖比例唯一性、后台查询与列表
- 风险高，验证成本高，回归面大

### Approach B: 前台多选，后端仍落单分组模板行，并增加模板组标识（采用）

做法：

- 管理页允许选择多个分组
- 保存时，后端在一个事务内展开成多条单分组模板
- 用 `bundle_key` 把这些行聚合成“一个模板组”
- 列表接口按 `bundle_key` 聚合返回给前端

优点：

- 运行时无需改动核心结算命中逻辑
- 后台配置体验达到“一个模板覆盖多个分组”
- 能保留现有单分组约束与测试资产
- 后续可继续在模板组层做联动编辑、联动启停、批量删除

代价：

- 模板管理接口要新增聚合读写能力
- 需要处理组内模板名、分组冲突、旧数据兼容

### Approach C: 只做前端循环创建多条模板，不引入模板组

做法：

- 前端多选分组
- 点击保存后，前端逐条请求创建多个模板

问题：

- 容易出现部分成功、部分失败
- 列表页无法自然聚合成一条管理记录
- 现有模板名全局唯一，会与展开后的多条记录冲突
- 后续编辑、删除、启停都很难保持一致

## Decision

采用 **Approach B**。

一句话定义本次能力：

- `管理端支持多分组模板组；运行时仍然只识别模板组展开后的单分组模板行`

## Current Constraints From Existing Implementation

当前实现对本次方案的直接约束如下：

- 模板只有单 `group` 字段
- 模板名当前是全局唯一，而不是按分组唯一
- 用户绑定唯一键是 `user_id + referral_type + group`
- 运行时只按单个 `group` 取活动模板

因此本次设计的原则是：

- 不改变运行时的“单 group 命中事实”
- 只在管理端增加模板组这一层抽象

## Data Model

### Referral Template Row

保留 `referral_templates` 的单分组结构，不把 `group` 改成数组。

新增字段：

- `bundle_key`
  - 类型建议：`varchar(64)`
  - 含义：同一模板组下所有模板行共享同一个 `bundle_key`
  - 用途：后台聚合展示、批量更新、批量删除

保留字段：

- `referral_type`
- `group`
- `name`
- `level_type`
- `enabled`
- `direct_cap_bps`
- `team_cap_bps`
- `invitee_share_default_bps`

### Bundle Semantics

模板组不是新的运行时实体，只是管理语义：

- 一个模板组可覆盖多个分组
- 一个模板组内部的每一条模板行都仍然只属于一个 `group`
- 一个模板组内部所有模板行的业务参数必须一致：
  - `referral_type`
  - `level_type`
  - `enabled`
  - `direct_cap_bps`
  - `team_cap_bps`
  - `invitee_share_default_bps`
- 组内允许的差异只有：
  - `id`
  - `group`
  - `bundle_key`
  - 组内派生模板名（如果采用）
  - 审计字段

### Name Uniqueness

当前模板名是全局唯一，这会阻止多分组展开。

本次改为：

- 模板名唯一性收敛到 `referral_type + group + name`

原因：

- 同一个模板组会展开成多个单分组模板行
- 如果模板名继续全局唯一，就无法对多个分组复用同一个“业务模板名”
- 将唯一性降到单分组粒度后，运行时语义不变，管理端也更符合直觉

### Backward Compatibility

旧数据兼容策略：

- 历史单分组模板默认视为“单成员模板组”
- 若旧行 `bundle_key` 为空，读取时可退化使用稳定派生值：
  - 首选迁移脚本补齐真实 `bundle_key`
  - 若迁移前读到空值，可临时按 `template:{id}` 聚合作为单成员组

## Write Model

### Create Bundle

创建请求以“模板组”为单位提交：

- `referral_type`
- `groups[]`
- `name`
- `level_type`
- `enabled`
- `direct_cap_bps`
- `team_cap_bps`
- `invitee_share_default_bps`

保存时：

1. 校验 `groups[]` 非空、去重、都属于现有系统分组
2. 生成新的 `bundle_key`
3. 在一个事务内为每个分组写入一条单分组模板行
4. 若任一分组冲突，则整个事务回滚

### Update Bundle

更新以 `bundle_key` 为单位，而不是单 `id`。

更新请求支持：

- 修改公共模板参数
- 替换整组 `groups[]`

更新过程：

1. 读出 `bundle_key` 下现有模板行
2. 比较旧分组集与新分组集
3. 事务内执行：
   - 对保留分组：更新公共参数
   - 对新增分组：创建新模板行
   - 对移除分组：删除对应模板行
4. 若任一新增分组冲突，则整体回滚

### Delete Bundle

删除动作以模板组为单位：

- 删除 `bundle_key` 下全部模板行

约束：

- 若后端当前已经允许删除被绑定模板，则保持现有语义
- 若后续要加“存在绑定时禁止删除”，也应在模板组层统一拦截

## Read Model

### List API

模板查询需要同时支持两种视图：

- `bundle view`
  - 给模板管理页使用
  - 返回聚合后的模板组视图
- `row view`
  - 给用户绑定、运行时排查、测试断言使用
  - 返回原始单分组模板行

首版不建议把旧接口默认值直接改成聚合视图，否则会影响现有绑定页。

返回结构建议：

- `bundle_key`
- `template_ids[]`
- `referral_type`
- `groups[]`
- `name`
- `level_type`
- `enabled`
- `direct_cap_bps`
- `team_cap_bps`
- `invitee_share_default_bps`
- `created_at`
- `updated_at`

排序建议：

- `referral_type`
- 第一分组名
- `name`

### Get/Expand Behavior

前端不需要在模板管理页感知组内每一条模板行，除非进入调试或绑定场景。

普通管理页：

- 只消费模板组视图

运行时及绑定相关接口：

- 继续消费单分组模板行

## Conflict Rules

### Group Occupancy

对于同一个 `referral_type`，同一个 `group` 下允许有多条模板行；这是当前已存在能力。

但模板组编辑时，本次只阻止“误把别的模板组已使用的同名管理槽位静默吞掉”。

具体规则：

- 同一个 `referral_type + group` 下本来就可以有多模板并存
- 但同一个模板组更新时，若某个新增分组会导致前端聚合结果不再可预测，则必须显式报错

首版采取更稳妥策略：

- 一个模板组写入某个 `referral_type + group + level_type + name` 时如果已被别的模板行占用，则报错
- 错误消息返回具体冲突分组

### Validation

保存时必须校验：

- `groups[]` 至少一个
- `groups[]` 无重复值
- 每个分组都存在于系统分组表
- 订阅返佣下：
  - `direct` 只能使用 `direct_cap_bps`
  - `team` 只能使用 `team_cap_bps`
- `invitee_share_default_bps` 在合法范围内

## Admin UI

### Template Editor

当前单选“分组”字段改成多选：

- 标签仍为“分组”
- 交互改成多选下拉
- 已选项展示为 tags

说明文案改成：

- 选择一个或多个已存在的系统分组
- 保存后会为每个分组生成一条运行时模板
- 结算和用户激活仍按 `返佣类型 + 单个分组` 命中

### Template List

列表按模板组展示：

- “分组”列展示多个 group tag
- 编辑时打开整个模板组，而不是单条模板行
- 删除时提示“会删除该模板组下所有分组模板”

### User Binding UI

首版不改绑定交互：

- 用户绑定仍然选具体模板
- 因为运行时仍是单分组模板行
- 绑定页继续请求 `row view`
- 当同一个模板组展开成多条同名模板行时，绑定下拉标签必须展示为“模板名 + 分组”，避免多个选项重名
- 后台若需要更强的一致体验，可在后续版本再把模板组选项抽象为更高层交互

## API Design

### Admin List Templates

保留现有路径：

- `GET /api/referral/templates`

行为调整：

- 默认保持兼容，继续返回 `row view`
- 新增查询参数 `view=bundle` 时返回模板组聚合结果
- 模板管理页切到 `view=bundle`
- 用户绑定页继续使用默认 `row view`

### Admin Create Template

保留现有路径：

- `POST /api/referral/templates`

请求从单 `group` 扩展为：

- 兼容旧参数 `group`
- 新增参数 `groups`

兼容策略：

- 若传 `groups[]`，按模板组创建
- 若只传单 `group`，视为单成员模板组

### Admin Update Template

保留现有路径：

- `PUT /api/referral/templates/:id`

行为调整：

- `:id` 代表模板组中的任一成员行
- 后端先根据该行解析出 `bundle_key`
- 再对整个模板组执行更新

### Admin Delete Template

保留现有路径：

- `DELETE /api/referral/templates/:id`

行为调整：

- `:id` 代表模板组中的任一成员行
- 后端删除其所属 `bundle_key` 下全部模板行

## Migration Strategy

需要一个数据库迁移：

1. 为 `referral_templates` 增加 `bundle_key`
2. 为历史模板逐条补齐 `bundle_key`
   - 每条旧模板默认独立成组
3. 调整模板名唯一性索引
4. 为 `bundle_key` 增加普通索引，便于聚合查询和批量更新

迁移原则：

- 不改旧模板的 `group`
- 不改旧绑定数据
- 不改旧结算记录和 override 数据

## Error Handling

错误文案应明确告诉管理员哪个分组失败。

示例：

- `以下分组保存失败：vip、agent`
- `分组 vip 下已存在同名模板`
- `分组 premium 不存在，请先在系统分组中创建`

不允许：

- 静默跳过某些分组
- 一部分分组成功、一部分失败但界面仍显示保存成功

## Testing Strategy

### Model / Controller

新增或更新测试覆盖：

- 单成员模板组创建
- 多分组模板组创建
- 更新模板组时新增分组
- 更新模板组时移除分组
- 更新模板组公共参数同步到全部成员行
- 冲突分组时事务回滚
- 旧单分组模板仍可正常读取成单成员模板组

### Frontend

新增或更新测试覆盖：

- 分组字段从单选改为多选
- 模板列表正确展示多个分组
- 编辑现有模板组能正确回填多分组选中值
- 保存时按新接口字段提交 `groups[]`
- 删除模板组时文案与行为正确

### Regression

重点回归：

- 用户绑定模板
- 返佣汇总页按 group 展示
- invitee override 读取当前 group 的模板默认值
- 订阅返佣结算入口仍按单 group 命中

## Rollout Plan

建议分两步上线：

1. 先加 `bundle_key`、兼容读写和模板组聚合接口
2. 再切换后台管理页到多分组编辑器

这样可以先完成后端兼容，再切前端交互，回滚也更容易。

## Success Criteria

上线后应满足：

- 管理员一次保存可覆盖多个分组
- 列表页仍只看到一条模板组记录，而不是多条重复模板
- 运行时结算结果与当前单分组逻辑一致
- 旧单分组模板无需人工迁移配置即可继续工作
- 新能力不会破坏用户绑定和 invitee override
