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
import ReferralFieldBlock from './ReferralFieldBlock';

const createDraftTemplate = () => ({
  id: `draft-${Date.now()}-${Math.random()}`,
  referralType: 'subscription_referral',
  group: '',
  name: '',
  levelType: 'direct',
  enabled: true,
  directCapBps: 1000,
  teamCapBps: 2500,
  teamDecayRatio: 0.5,
  teamMaxDepth: 3,
  inviteeShareDefaultBps: 0,
  isDraft: true,
});

const formatBpsAsPercent = (value) => {
  const normalized = Number(value || 0) / 100;
  if (Number.isInteger(normalized)) {
    return `${normalized}%`;
  }
  return `${normalized.toFixed(2).replace(/\.?0+$/, '')}%`;
};

const ruleSections = [
  {
    title: '入口判定',
    items: [
      '第一层直接邀请人没有活动模板：本单不返佣。',
      '第一层是 team：只结算最近这个 team。',
      '第一层是 direct：先结直推；命中有效 team 后，才触发团队级差分配。',
    ],
  },
  {
    title: '向上遍历',
    items: [
      '上层没有模板或模板未启用：跳过，但不断链。',
      '上层是 direct：不拿第二份返佣。',
      '上层是 team：参与团队级差分配。',
    ],
  },
  {
    title: '切分与回补',
    items: [
      'invitee reward 只从最近直接邀请人的那份里切出。',
      'team_reward 不会再切给付款用户。',
      '没命中任何有效 team 时，本单不成立团队级差返佣。',
    ],
  },
];

const SettingsReferralTemplates = () => {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(false);
  const [savingKey, setSavingKey] = useState('');
  const [deletingKey, setDeletingKey] = useState('');

  const referralTypeOptions = [
    { label: 'subscription_referral', value: 'subscription_referral' },
  ];
  const levelTypeOptions = [
    { label: t('direct'), value: 'direct' },
    { label: t('team'), value: 'team' },
  ];

  const load = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/referral/templates');
      if (res.data?.success) {
        setItems(normalizeReferralTemplateItems(res.data?.data?.items));
      } else {
        showError(res.data?.message || t('加载失败'));
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
      currentItems.map((item) => (item.id === id ? { ...item, ...patch } : item)),
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
        group: row.group,
        name: row.name,
        level_type: row.levelType,
        enabled: row.enabled,
        direct_cap_bps: Number(row.directCapBps || 0),
        team_cap_bps: Number(row.teamCapBps || 0),
        team_decay_ratio: Number(row.teamDecayRatio || 0),
        team_max_depth: Number(row.teamMaxDepth || 0),
        invitee_share_default_bps: Number(row.inviteeShareDefaultBps || 0),
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
      setItems((currentItems) => currentItems.filter((item) => item.id !== row.id));
      return;
    }
    if (!window.confirm(t('确认删除该返佣模板？'))) {
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

  return (
    <div className='space-y-3'>
      <div className='flex items-center justify-between gap-3'>
        <div>
          <Typography.Title heading={5} style={{ marginBottom: 0 }}>
            {t('返佣模板')}
          </Typography.Title>
          <Typography.Text type='secondary'>
            {t('管理返佣类型与分组下的模板配置。')}
          </Typography.Text>
        </div>
        <Button type='primary' onClick={addDraft}>
          {t('新增模板')}
        </Button>
      </div>
      <Banner
        type='info'
        bordered
        title={t('填写说明')}
        description={t(
          '所有 bps 字段都按万分比填写：10000 = 100%，1000 = 10%。direct 表示最近直接邀请人先拿直推，只有命中有效 team 后才会触发团队级差分配；team 表示最近邀请人直接按团队模板结算。切到 template 引擎前，先确认这个分组已经绑定了用户模板。',
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
                    <Typography.Text type='secondary'>{t(item)}</Typography.Text>
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
          items.map((row) => (
            <div key={row.id} className='rounded-xl border border-gray-200 p-4 space-y-3'>
              <div className='grid grid-cols-1 gap-3 lg:grid-cols-3'>
                <ReferralFieldBlock
                  label={t('返佣类型')}
                  description={t('当前模板属于哪个返佣体系。当前页面只支持 subscription_referral。')}
                >
                  <Select
                    value={row.referralType}
                    optionList={referralTypeOptions}
                    onChange={(value) => updateRow(row.id, { referralType: value })}
                  />
                </ReferralFieldBlock>
                <ReferralFieldBlock
                  label={t('分组')}
                  description={t('必须与订阅计划分组一致。结算时按 referral_type + group 命中模板。')}
                >
                  <Input
                    value={row.group}
                    placeholder={t('分组')}
                    onChange={(value) => updateRow(row.id, { group: value })}
                  />
                </ReferralFieldBlock>
                <ReferralFieldBlock
                  label={t('模板名')}
                  description={t('只用于后台识别，不参与返佣计算。建议按分组和身份命名。')}
                >
                  <Input
                    value={row.name}
                    placeholder={t('模板名')}
                    onChange={(value) => updateRow(row.id, { name: value })}
                  />
                </ReferralFieldBlock>
              </div>
              <div className='grid grid-cols-1 gap-3 lg:grid-cols-4'>
                <ReferralFieldBlock
                  label={t('模板身份')}
                  description={t('direct 会先结算最近直接邀请人；只有命中有效 team 后，才会向上做团队级差分配。team 会让最近邀请人直接按团队模板结算。')}
                >
                  <Select
                    value={row.levelType}
                    optionList={levelTypeOptions}
                    onChange={(value) => updateRow(row.id, { levelType: value })}
                  />
                </ReferralFieldBlock>
                <ReferralFieldBlock
                  label={t('直推上限比例')}
                  description={t('最近 direct 邀请人那一份的毛额比例。只在模板身份为 direct 时直接生效。')}
                  note={t('当前约 {{value}}', { value: formatBpsAsPercent(row.directCapBps) })}
                >
                  <InputNumber
                    value={row.directCapBps}
                    min={0}
                    max={10000}
                    step={100}
                    style={{ width: '100%' }}
                    onChange={(value) => updateRow(row.id, { directCapBps: Number(value || 0) })}
                  />
                </ReferralFieldBlock>
                <ReferralFieldBlock
                  label={t('团队总上限比例')}
                  description={t('整单返佣总上限。若当前模板是 direct，则直推与命中 team 后成立的团队级差合计不超过它；若模板是 team，则最近邀请人直接按它结算。')}
                  note={t('当前约 {{value}}', { value: formatBpsAsPercent(row.teamCapBps) })}
                >
                  <InputNumber
                    value={row.teamCapBps}
                    min={0}
                    max={10000}
                    step={100}
                    style={{ width: '100%' }}
                    onChange={(value) => updateRow(row.id, { teamCapBps: Number(value || 0) })}
                  />
                </ReferralFieldBlock>
                <ReferralFieldBlock
                  label={t('被邀请人默认返佣比例')}
                  description={t('默认从最近直接邀请人的毛额里切多少给付款用户本人。0 表示不返给被邀请人。')}
                  detail={t(
                    '实际生效优先级：单个 invitee 覆盖 > 用户绑定默认值 > 模板默认值。',
                  )}
                  note={t('当前约 {{value}}', {
                    value: formatBpsAsPercent(row.inviteeShareDefaultBps),
                  })}
                >
                  <InputNumber
                    value={row.inviteeShareDefaultBps}
                    min={0}
                    max={10000}
                    step={100}
                    style={{ width: '100%' }}
                    onChange={(value) =>
                      updateRow(row.id, { inviteeShareDefaultBps: Number(value || 0) })
                    }
                  />
                </ReferralFieldBlock>
              </div>
              <div className='grid grid-cols-1 gap-3 lg:grid-cols-3'>
                <ReferralFieldBlock
                  label={t('团队衰减系数')}
                  description={t('仅在 direct 模式且命中有效 team 节点后使用。越小越偏向近层团队节点，例如 0.5 代表每多一层权重减半。')}
                  note={t('当前 {{value}}', { value: Number(row.teamDecayRatio || 0) })}
                >
                  <InputNumber
                    value={row.teamDecayRatio}
                    min={0}
                    max={1}
                    step={0.1}
                    style={{ width: '100%' }}
                    onChange={(value) => updateRow(row.id, { teamDecayRatio: Number(value || 0) })}
                  />
                </ReferralFieldBlock>
                <ReferralFieldBlock
                  label={t('团队最大深度')}
                  description={t('仅在 direct 模式生效，表示最多向上遍历多少层真实邀请关系。超过这个深度的 team 节点不参与分配。')}
                  note={t('当前最多 {{count}} 层', {
                    count: Number(row.teamMaxDepth || 1),
                  })}
                >
                  <InputNumber
                    value={row.teamMaxDepth}
                    min={1}
                    step={1}
                    style={{ width: '100%' }}
                    onChange={(value) => updateRow(row.id, { teamMaxDepth: Number(value || 1) })}
                  />
                </ReferralFieldBlock>
                <ReferralFieldBlock
                  label={t('启用模板')}
                  description={t('关闭后，该模板不会被解析为活动模板。即使用户已经绑定，也不会参与新模板返佣结算。')}
                >
                  <div className='flex items-center gap-2 pt-2'>
                    <Typography.Text>{t('启用')}</Typography.Text>
                    <Switch
                      checked={row.enabled}
                      onChange={(checked) => updateRow(row.id, { enabled: checked })}
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
          ))
        )}
      </Spin>
    </div>
  );
};

export default SettingsReferralTemplates;
