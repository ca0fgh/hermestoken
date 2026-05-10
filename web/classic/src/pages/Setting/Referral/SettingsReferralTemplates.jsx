/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import React, { useEffect, useState } from 'react';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';
import {
  Banner,
  Button,
  Empty,
  Input,
  InputNumber,
  Select,
  Space,
  Spin,
  Switch,
  Typography,
} from '@douyinfe/semi-ui';
import { normalizeReferralTemplateItems } from '../../../helpers/referralTemplate';
import {
  buildReferralLevelTypeOptions,
  buildReferralTypeOptions,
} from '../../../helpers/referralLabels';
import {
  percentNumberToRateBps,
  rateBpsToPercentNumber,
} from '../../../helpers/subscriptionReferral';
import ReferralFieldBlock from './ReferralFieldBlock';

const createDraftTemplate = () => ({
  id: `draft-${Date.now()}-${Math.random()}`,
  bundleKey: '',
  templateIds: [],
  referralType: 'subscription_referral',
  groups: [],
  name: '',
  levelType: 'direct',
  enabled: true,
  directCapBps: 1000,
  teamCapBps: 0,
  inviteeShareDefaultBps: 0,
  groupRates: [],
  isDraft: true,
});

const formatBpsAsPercent = (value) => {
  const normalized = Number(value || 0) / 100;
  if (Number.isInteger(normalized)) {
    return `${normalized}%`;
  }
  return `${normalized.toFixed(2).replace(/\.?0+$/, '')}%`;
};

const referralTemplateGroupRateForGroup = (row, group) => {
  const trimmedGroup = String(group || '').trim();
  const existingRate = (row.groupRates || []).find(
    (rate) => rate.group === trimmedGroup,
  );
  return {
    group: trimmedGroup,
    directCapBps: Number(existingRate?.directCapBps ?? row.directCapBps ?? 0),
    teamCapBps: Number(existingRate?.teamCapBps ?? row.teamCapBps ?? 0),
    inviteeShareDefaultBps: Number(
      existingRate?.inviteeShareDefaultBps ?? row.inviteeShareDefaultBps ?? 0,
    ),
  };
};

const sortReferralTemplateGroupRates = (groupRates = []) =>
  [...groupRates].sort((left, right) => left.group.localeCompare(right.group));

const mergeReferralTemplateGroupRate = (row, group, patch) => {
  const trimmedGroup = String(group || '').trim();
  if (!trimmedGroup) {
    return row.groupRates || [];
  }
  const nextRate = {
    ...referralTemplateGroupRateForGroup(row, trimmedGroup),
    ...patch,
    group: trimmedGroup,
  };
  return sortReferralTemplateGroupRates([
    ...(row.groupRates || []).filter((rate) => rate.group !== trimmedGroup),
    nextRate,
  ]);
};

const mergeReferralTemplateGroupRateForGroups = (row, groups, patch) =>
  (groups || []).reduce(
    (groupRates, group) =>
      mergeReferralTemplateGroupRate({ ...row, groupRates }, group, patch),
    row.groupRates || [],
  );

const referralTemplateIdentityRateBps = (row, group) => {
  const groupRate = referralTemplateGroupRateForGroup(row, group);
  return row.levelType === 'team'
    ? groupRate.teamCapBps
    : groupRate.directCapBps;
};

const referralTemplateInviteeShareDefaultBps = (row, group) =>
  referralTemplateGroupRateForGroup(row, group).inviteeShareDefaultBps;

const referralTemplateInviteeShareDefaultPatch = (row, rateBps) => ({
  inviteeShareDefaultBps: rateBps,
  groupRates: mergeReferralTemplateGroupRateForGroups(row, row.groups, {
    inviteeShareDefaultBps: rateBps,
  }),
});

const referralTemplateIdentityRatePatch = (row, rateBps) =>
  row.levelType === 'team'
    ? {
        teamCapBps: rateBps,
        groupRates: mergeReferralTemplateGroupRateForGroups(row, row.groups, {
          teamCapBps: rateBps,
        }),
      }
    : {
        directCapBps: rateBps,
        groupRates: mergeReferralTemplateGroupRateForGroups(row, row.groups, {
          directCapBps: rateBps,
        }),
      };

const buildReferralTemplateGroupRatesPayload = (row) =>
  (row.groups || []).map((group) => {
    const groupRate = referralTemplateGroupRateForGroup(row, group);
    return {
      group,
      direct_cap_bps:
        row.levelType === 'direct' ? Number(groupRate.directCapBps || 0) : 0,
      team_cap_bps:
        row.levelType === 'team' ? Number(groupRate.teamCapBps || 0) : 0,
      invitee_share_default_bps: Number(groupRate.inviteeShareDefaultBps || 0),
    };
  });

const ReferralTemplateRateInput = ({ valueBps, onChange }) => (
  <div className='flex min-w-0 items-center gap-2'>
    <Typography.Text
      type='secondary'
      className='w-12 shrink-0 whitespace-nowrap text-right'
    >
      {formatBpsAsPercent(valueBps)}
    </Typography.Text>
    <InputNumber
      value={rateBpsToPercentNumber(valueBps)}
      min={0}
      max={100}
      step={0.1}
      suffix='%'
      className='min-w-0 flex-1'
      style={{ width: '100%' }}
      onChange={(value) => onChange(percentNumberToRateBps(value))}
    />
  </div>
);

const ReferralTemplateRateColumn = ({ label, valueBps, onChange }) => (
  <div className='min-w-0 space-y-1'>
    <Typography.Text type='tertiary' className='block lg:hidden'>
      {label}
    </Typography.Text>
    <ReferralTemplateRateInput valueBps={valueBps} onChange={onChange} />
  </div>
);

const ReferralTemplateGroupRatesEditor = ({
  t,
  row,
  identityRateLabel,
  identityRateBps,
  onInviteeBulkChange,
  onInviteeGroupChange,
  onIdentityBulkChange,
  onIdentityGroupChange,
}) => {
  const inviteeRateLabel = t('被邀请人默认返佣比例');

  return (
    <div className='overflow-hidden rounded-lg border border-gray-200 bg-white'>
      <div className='hidden grid-cols-[minmax(0,1fr)_minmax(200px,260px)_minmax(200px,260px)] gap-3 border-b border-gray-100 bg-gray-50/70 px-3 py-2.5 lg:grid lg:items-center'>
        <Typography.Text type='tertiary'>{t('分组')}</Typography.Text>
        <Typography.Text type='tertiary'>{inviteeRateLabel}</Typography.Text>
        <Typography.Text type='tertiary'>{identityRateLabel}</Typography.Text>
      </div>
      <div className='grid grid-cols-1 gap-2 border-b border-gray-100 bg-gray-50/70 px-3 py-2.5 lg:grid-cols-[minmax(0,1fr)_minmax(200px,260px)_minmax(200px,260px)] lg:items-center'>
        <div className='min-w-0'>
          <Typography.Text strong className='block truncate'>
            {t('全部分组批量设置')}
          </Typography.Text>
          <Typography.Text type='tertiary' className='block'>
            {t('修改后同步覆盖所有分组，可继续逐项微调')}
          </Typography.Text>
        </div>
        <ReferralTemplateRateColumn
          label={inviteeRateLabel}
          valueBps={row.inviteeShareDefaultBps}
          onChange={onInviteeBulkChange}
        />
        <ReferralTemplateRateColumn
          label={identityRateLabel}
          valueBps={identityRateBps}
          onChange={onIdentityBulkChange}
        />
      </div>
      <div className='divide-y divide-gray-100'>
        {row.groups.map((group) => (
          <div
            key={group}
            className='grid grid-cols-1 gap-2 px-3 py-2.5 lg:grid-cols-[minmax(0,1fr)_minmax(200px,260px)_minmax(200px,260px)] lg:items-center'
          >
            <Typography.Text
              strong
              className='block min-w-0 truncate'
              title={group}
            >
              {group}
            </Typography.Text>
            <ReferralTemplateRateColumn
              label={inviteeRateLabel}
              valueBps={referralTemplateInviteeShareDefaultBps(row, group)}
              onChange={(rateBps) => onInviteeGroupChange(group, rateBps)}
            />
            <ReferralTemplateRateColumn
              label={identityRateLabel}
              valueBps={referralTemplateIdentityRateBps(row, group)}
              onChange={(rateBps) => onIdentityGroupChange(group, rateBps)}
            />
          </div>
        ))}
      </div>
    </div>
  );
};

const ruleSections = [
  {
    title: '入口判定',
    items: [
      '第一层直接邀请人没有活动模板：本单不返佣。',
      '第一层是 team：只结算最近这个 team。',
      '第一层是 direct：先结直推；继续向上找到第一个有效 team 后，才成立团队池。',
    ],
  },
  {
    title: '向上遍历',
    items: [
      '上层没有模板或模板未启用：跳过，但不断链。',
      '上层是 direct：不拿第二份返佣。',
      '上层是 team：只要命中，就参与同一个团队池分配。',
    ],
  },
  {
    title: '团队池',
    items: [
      'invitee reward 只从最近邀请人的即时返佣里切出，不会从 team_reward 池里切。',
      'team_reward 不会再切给付款用户。',
      '团队池按“首个命中 team 的比例 - direct 直推比例”成立；没命中任何有效 team 时，本单不成立团队级差返佣。',
    ],
  },
];

const SettingsReferralTemplates = () => {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [subscriptionSetting, setSubscriptionSetting] = useState({
    teamDecayRatio: 0.5,
    teamMaxDepth: 0,
    autoAssignInviteeTemplate: true,
    planOpenToAllUsers: false,
  });
  const [groupOptions, setGroupOptions] = useState([]);
  const [loading, setLoading] = useState(false);
  const [savingGlobalSetting, setSavingGlobalSetting] = useState(false);
  const [savingKey, setSavingKey] = useState('');
  const [deletingKey, setDeletingKey] = useState('');

  const referralTypeOptions = buildReferralTypeOptions(t);
  const levelTypeOptions = buildReferralLevelTypeOptions(t);

  const load = async () => {
    setLoading(true);
    try {
      const [templateRes, groupRes, settingRes] = await Promise.all([
        API.get('/api/referral/templates', { params: { view: 'bundle' } }),
        API.get('/api/group'),
        API.get('/api/referral/settings/subscription'),
      ]);
      if (templateRes.data?.success) {
        setItems(normalizeReferralTemplateItems(templateRes.data?.data?.items));
      } else {
        showError(templateRes.data?.message || t('加载失败'));
      }

      if (groupRes.data?.success) {
        setGroupOptions(
          (groupRes.data?.data || []).map((group) => ({
            label: group,
            value: group,
          })),
        );
      } else {
        setGroupOptions([]);
      }

      if (settingRes.data?.success) {
        setSubscriptionSetting({
          teamDecayRatio: Number(
            settingRes.data?.data?.team_decay_ratio || 0.5,
          ),
          teamMaxDepth: Number(settingRes.data?.data?.team_max_depth || 0),
          autoAssignInviteeTemplate:
            settingRes.data?.data?.auto_assign_invitee_template !== false,
          planOpenToAllUsers:
            settingRes.data?.data?.plan_open_to_all_users === true,
        });
      }
    } catch (error) {
      showError(error?.message || t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, [t]);

  const updateRow = (id, patch) => {
    setItems((currentItems) =>
      currentItems.map((item) =>
        item.id === id ? { ...item, ...patch } : item,
      ),
    );
  };

  const updateReferralTemplateGroupRate = (id, group, patch) => {
    setItems((currentItems) =>
      currentItems.map((item) =>
        item.id === id
          ? {
              ...item,
              groupRates: mergeReferralTemplateGroupRate(item, group, patch),
            }
          : item,
      ),
    );
  };

  const addDraft = () => {
    setItems((currentItems) => [...currentItems, createDraftTemplate()]);
  };

  const saveRow = async (row) => {
    const key = String(row.id);
    setSavingKey(key);
    try {
      const payload = {
        referral_type: row.referralType,
        groups: row.groups,
        name: row.name,
        level_type: row.levelType,
        enabled: row.enabled,
        direct_cap_bps:
          row.levelType === 'direct' ? Number(row.directCapBps || 0) : 0,
        team_cap_bps:
          row.levelType === 'team' ? Number(row.teamCapBps || 0) : 0,
        invitee_share_default_bps: Number(row.inviteeShareDefaultBps || 0),
        group_rates: buildReferralTemplateGroupRatesPayload(row),
      };
      const res = row.isDraft
        ? await API.post('/api/referral/templates', payload)
        : await API.put(`/api/referral/templates/${row.id}`, payload);

      if (res.data?.success) {
        showSuccess(t('保存成功'));
        await load();
      } else {
        showError(res.data?.message || t('保存失败'));
      }
    } catch (error) {
      showError(error?.message || t('保存失败'));
    } finally {
      setSavingKey('');
    }
  };

  const deleteRow = async (row) => {
    if (row.isDraft) {
      setItems((currentItems) =>
        currentItems.filter((item) => item.id !== row.id),
      );
      return;
    }
    if (!window.confirm(t('确认删除该模板组及其覆盖的所有分组模板吗？'))) {
      return;
    }
    const key = String(row.id);
    setDeletingKey(key);
    try {
      const res = await API.delete(`/api/referral/templates/${row.id}`);
      if (res.data?.success) {
        showSuccess(t('删除成功'));
        await load();
      } else {
        showError(res.data?.message || t('删除失败'));
      }
    } catch (error) {
      showError(error?.message || t('删除失败'));
    } finally {
      setDeletingKey('');
    }
  };

  const saveSubscriptionSetting = async () => {
    setSavingGlobalSetting(true);
    try {
      const res = await API.put('/api/referral/settings/subscription', {
        team_decay_ratio: Number(subscriptionSetting.teamDecayRatio || 0),
        team_max_depth: Number(subscriptionSetting.teamMaxDepth || 0),
        auto_assign_invitee_template: Boolean(
          subscriptionSetting.autoAssignInviteeTemplate,
        ),
        plan_open_to_all_users: Boolean(subscriptionSetting.planOpenToAllUsers),
      });
      if (res.data?.success) {
        showSuccess(t('保存成功'));
        setSubscriptionSetting({
          teamDecayRatio: Number(res.data?.data?.team_decay_ratio || 0.5),
          teamMaxDepth: Number(res.data?.data?.team_max_depth || 0),
          autoAssignInviteeTemplate:
            res.data?.data?.auto_assign_invitee_template !== false,
          planOpenToAllUsers: res.data?.data?.plan_open_to_all_users === true,
        });
      } else {
        showError(res.data?.message || t('保存失败'));
      }
    } catch (error) {
      showError(error?.message || t('保存失败'));
    } finally {
      setSavingGlobalSetting(false);
    }
  };

  const getLevelTypeDescription = (levelType) => {
    if (levelType === 'team') {
      return t(
        'team 会让最近邀请人直接按团队模板结算；如果它不是最近邀请人，而是向上链路中的 team，则只用自己的团队比例参与定池或分池。',
      );
    }
    return t(
      'direct 会先结算最近直接邀请人；它不再配置团队池上限，团队池改由向上命中的第一个有效 team 决定。',
    );
  };

  return (
    <div className='space-y-3'>
      <div className='flex items-center justify-between gap-3'>
        <div>
          <Typography.Title heading={5} style={{ marginBottom: 0 }}>
            {t('返佣模板')}
          </Typography.Title>
          <Typography.Text type='secondary'>
            {t(
              '管理返佣模板组；一个模板组可以覆盖多个系统分组，但每个分组在运行时仍命中各自的单分组模板行。',
            )}
          </Typography.Text>
        </div>
        <Button type='primary' onClick={addDraft}>
          {t('新增模板组')}
        </Button>
      </div>
      <div className='rounded-xl border border-gray-200 bg-white p-4 space-y-3'>
        <div className='flex items-center justify-between gap-3'>
          <div>
            <Typography.Text strong>{t('订阅返佣全局设置')}</Typography.Text>
            <div>
              <Typography.Text type='secondary'>
                {t(
                  '这些参数对 subscription_referral 的整条团队返佣链统一生效，不属于单个模板。',
                )}
              </Typography.Text>
            </div>
          </div>
          <Button
            type='primary'
            loading={savingGlobalSetting}
            onClick={saveSubscriptionSetting}
          >
            {t('保存')}
          </Button>
        </div>
        <div className='grid grid-cols-1 gap-3 lg:grid-cols-2 xl:grid-cols-4'>
          <ReferralFieldBlock
            label={t('邀请注册自动开通返佣资格')}
            description={t(
              '开启后，被已有订阅返佣资格的用户邀请注册的新用户，会自动绑定当前最低档订阅返佣模板。关闭后，新用户不会自动获得订阅返佣资格，需要管理员手动绑定。',
            )}
            note={
              subscriptionSetting.autoAssignInviteeTemplate
                ? t('当前开启')
                : t('当前关闭')
            }
          >
            <Switch
              checked={subscriptionSetting.autoAssignInviteeTemplate}
              onChange={(checked) =>
                setSubscriptionSetting((currentSetting) => ({
                  ...currentSetting,
                  autoAssignInviteeTemplate: checked,
                }))
              }
            />
          </ReferralFieldBlock>
          <ReferralFieldBlock
            label={t('订阅套餐开放给所有用户')}
            description={t(
              '开启后，所有已登录用户都可以看到并购买订阅套餐。关闭后，只有已开通订阅返佣资格的用户可以看到并购买。',
            )}
            note={
              subscriptionSetting.planOpenToAllUsers
                ? t('当前开启')
                : t('当前关闭')
            }
          >
            <Switch
              checked={subscriptionSetting.planOpenToAllUsers}
              onChange={(checked) =>
                setSubscriptionSetting((currentSetting) => ({
                  ...currentSetting,
                  planOpenToAllUsers: checked,
                }))
              }
            />
          </ReferralFieldBlock>
          <ReferralFieldBlock
            label={t('团队衰减系数')}
            description={t(
              '这是订阅返佣的全局参数。命中有效 team 后，会对整条团队级差分配链统一生效。越小越偏向近层团队节点，例如 0.5 代表每多一层权重减半。',
            )}
            note={t('当前 {{value}}', {
              value: Number(subscriptionSetting.teamDecayRatio || 0),
            })}
          >
            <InputNumber
              value={subscriptionSetting.teamDecayRatio}
              min={0}
              max={1}
              step={0.1}
              style={{ width: '100%' }}
              onChange={(value) =>
                setSubscriptionSetting((currentSetting) => ({
                  ...currentSetting,
                  teamDecayRatio: Number(value || 0),
                }))
              }
            />
          </ReferralFieldBlock>
          <ReferralFieldBlock
            label={t('团队最大深度')}
            description={t(
              '这是订阅返佣的全局参数，对所有 direct 入场后触发的团队返佣链统一生效。默认值为 0，表示不限深度。超过这个深度的 team 节点不参与分配。',
            )}
            note={
              subscriptionSetting.teamMaxDepth > 0
                ? t('当前最多 {{count}} 层', {
                    count: Number(subscriptionSetting.teamMaxDepth),
                  })
                : t('当前不限深度')
            }
          >
            <InputNumber
              value={subscriptionSetting.teamMaxDepth}
              min={0}
              step={1}
              style={{ width: '100%' }}
              onChange={(value) =>
                setSubscriptionSetting((currentSetting) => ({
                  ...currentSetting,
                  teamMaxDepth: Number(value || 0),
                }))
              }
            />
          </ReferralFieldBlock>
        </div>
      </div>
      <Banner
        type='info'
        bordered
        title={t('填写说明')}
        description={t(
          '比例字段按百分比输入，保存时会自动换算成 bps：10 表示 10%，25 表示 25%。一个模板组可以覆盖多个系统分组，保存时会按 bundle 一次性更新所有关联分组。模板名只需要在同一返佣类型 + 分组内保持唯一。direct 只配置最近直接邀请人的直推比例；命中有效 team 后，团队池按“首个命中 team 的比例 - direct 直推比例”成立，再由所有命中的 team 按全局权重分配。team 表示最近邀请人直接按团队模板结算。团队衰减系数和团队最大深度在订阅返佣全局设置里统一配置，不再跟着单个模板走。',
        )}
      />
      <div className='rounded-xl border border-gray-200 bg-white p-4 space-y-3'>
        <div>
          <Typography.Text strong>{t('关键规则')}</Typography.Text>
          <div>
            <Typography.Text type='secondary'>
              {t('只展示管理员配置时最容易误解的 3 条规则。')}
            </Typography.Text>
          </div>
        </div>
        <div className='grid grid-cols-1 gap-3 xl:grid-cols-3'>
          {ruleSections.map((section) => (
            <div
              key={section.title}
              className='rounded-xl border border-gray-200 bg-gray-50/60 p-3 space-y-2'
            >
              <Typography.Text strong>{t(section.title)}</Typography.Text>
              <div className='space-y-1'>
                {section.items.map((item) => (
                  <div key={item}>
                    <Typography.Text type='secondary'>
                      {t(item)}
                    </Typography.Text>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
      <Spin spinning={loading}>
        {items.length === 0 ? (
          <Empty description={t('暂无返佣模板')} />
        ) : (
          items.map((row) => {
            const isDirectTemplate = row.levelType === 'direct';
            const identityRateLabel = isDirectTemplate
              ? t('直推返佣比例')
              : t('团队返佣比例');
            const identityRateBps = isDirectTemplate
              ? row.directCapBps
              : row.teamCapBps;

            return (
              <div
                key={row.id}
                className='rounded-xl border border-gray-200 p-4 space-y-3'
              >
                <div className='grid grid-cols-1 gap-3 lg:grid-cols-12'>
                  <ReferralFieldBlock
                    className='lg:col-span-4'
                    label={t('返佣类型')}
                    description={t(
                      '当前模板属于哪个返佣体系。当前页面只支持订阅返佣。',
                    )}
                  >
                    <Select
                      value={row.referralType}
                      optionList={referralTypeOptions}
                      style={{ width: '100%' }}
                      onChange={(value) =>
                        updateRow(row.id, { referralType: value })
                      }
                    />
                  </ReferralFieldBlock>
                  <ReferralFieldBlock
                    className='lg:col-span-8'
                    label={t('分组')}
                    description={t(
                      '必须选择至少一个已存在的系统分组。保存后会为每个分组维护一条运行时模板行。',
                    )}
                  >
                    <Select
                      value={row.groups}
                      multiple={true}
                      optionList={groupOptions}
                      placeholder={t('分组')}
                      style={{ width: '100%' }}
                      onChange={(value) =>
                        updateRow(row.id, {
                          groups: Array.isArray(value)
                            ? value
                                .map((group) => String(group || '').trim())
                                .filter(Boolean)
                            : [],
                        })
                      }
                    />
                  </ReferralFieldBlock>
                  <ReferralFieldBlock
                    className='lg:col-span-6'
                    label={t('模板名')}
                    description={t(
                      '只用于后台识别，不参与返佣计算。模板名只需要在同一返佣类型 + 分组内保持唯一，建议按业务含义和模板身份命名。',
                    )}
                  >
                    <Input
                      value={row.name}
                      placeholder={t('模板名')}
                      onChange={(value) => updateRow(row.id, { name: value })}
                    />
                  </ReferralFieldBlock>
                  <ReferralFieldBlock
                    className='lg:col-span-6'
                    label={t('模板身份')}
                    description={getLevelTypeDescription(row.levelType)}
                  >
                    <Select
                      value={row.levelType}
                      optionList={levelTypeOptions}
                      style={{ width: '100%' }}
                      onChange={(value) =>
                        updateRow(row.id, {
                          levelType: value,
                          directCapBps:
                            value === 'team'
                              ? 0
                              : row.directCapBps > 0
                                ? row.directCapBps
                                : 1000,
                          teamCapBps:
                            value === 'team'
                              ? row.teamCapBps > 0
                                ? row.teamCapBps
                                : 2500
                              : 0,
                          groupRates: row.groupRates || [],
                        })
                      }
                    />
                  </ReferralFieldBlock>
                </div>
                <ReferralFieldBlock
                  label={t('分组返佣比例')}
                  description={t(
                    '按分组同时设置付款用户本人默认比例和模板身份返佣比例；需要统一调整时，先改全部分组行，再单独改个别分组。',
                  )}
                  detail={t(
                    '实际生效优先级：单个 invitee 覆盖 > 用户绑定默认值 > 模板默认值；保存后会按订单命中的分组读取对应模板行。',
                  )}
                >
                  {row.groups.length > 0 ? (
                    <ReferralTemplateGroupRatesEditor
                      t={t}
                      row={row}
                      identityRateLabel={identityRateLabel}
                      identityRateBps={identityRateBps}
                      onInviteeBulkChange={(rateBps) =>
                        updateRow(
                          row.id,
                          referralTemplateInviteeShareDefaultPatch(
                            row,
                            rateBps,
                          ),
                        )
                      }
                      onInviteeGroupChange={(group, rateBps) =>
                        updateReferralTemplateGroupRate(row.id, group, {
                          inviteeShareDefaultBps: rateBps,
                        })
                      }
                      onIdentityBulkChange={(rateBps) =>
                        updateRow(
                          row.id,
                          referralTemplateIdentityRatePatch(row, rateBps),
                        )
                      }
                      onIdentityGroupChange={(group, rateBps) =>
                        updateReferralTemplateGroupRate(
                          row.id,
                          group,
                          isDirectTemplate
                            ? {
                                directCapBps: rateBps,
                              }
                            : {
                                teamCapBps: rateBps,
                              },
                        )
                      }
                    />
                  ) : (
                    <Typography.Text type='secondary'>
                      {t('选择分组后可分别设置比例')}
                    </Typography.Text>
                  )}
                </ReferralFieldBlock>
                <div className='grid grid-cols-1 gap-3 lg:grid-cols-1'>
                  <ReferralFieldBlock
                    label={t('启用模板')}
                    description={t(
                      '关闭后，该模板不会被解析为活动模板。即使用户已经绑定，也不会参与新模板返佣结算。',
                    )}
                  >
                    <div className='flex items-center gap-2 pt-2'>
                      <Typography.Text>{t('启用')}</Typography.Text>
                      <Switch
                        checked={row.enabled}
                        onChange={(checked) =>
                          updateRow(row.id, { enabled: checked })
                        }
                      />
                    </div>
                  </ReferralFieldBlock>
                </div>
                <Space>
                  <Button
                    type='primary'
                    loading={savingKey === String(row.id)}
                    onClick={() => saveRow(row)}
                  >
                    {t('保存')}
                  </Button>
                  <Button
                    type='danger'
                    theme='borderless'
                    loading={deletingKey === String(row.id)}
                    onClick={() => deleteRow(row)}
                  >
                    {t('删除')}
                  </Button>
                </Space>
              </div>
            );
          })
        )}
      </Spin>
    </div>
  );
};

export default SettingsReferralTemplates;
