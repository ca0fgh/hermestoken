import React, { useEffect, useState } from 'react';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';
import {
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
      <Spin spinning={loading}>
        {items.length === 0 ? (
          <Empty description={t('暂无返佣模板')} />
        ) : (
          items.map((row) => (
            <div key={row.id} className='rounded-xl border border-gray-200 p-4 space-y-3'>
              <div className='grid grid-cols-1 gap-3 lg:grid-cols-3'>
                <Select
                  value={row.referralType}
                  optionList={referralTypeOptions}
                  onChange={(value) => updateRow(row.id, { referralType: value })}
                />
                <Input
                  value={row.group}
                  placeholder={t('分组')}
                  onChange={(value) => updateRow(row.id, { group: value })}
                />
                <Input
                  value={row.name}
                  placeholder={t('模板名')}
                  onChange={(value) => updateRow(row.id, { name: value })}
                />
              </div>
              <div className='grid grid-cols-1 gap-3 lg:grid-cols-4'>
                <Select
                  value={row.levelType}
                  optionList={levelTypeOptions}
                  onChange={(value) => updateRow(row.id, { levelType: value })}
                />
                <InputNumber
                  value={row.directCapBps}
                  min={0}
                  max={10000}
                  step={100}
                  style={{ width: '100%' }}
                  onChange={(value) => updateRow(row.id, { directCapBps: Number(value || 0) })}
                />
                <InputNumber
                  value={row.teamCapBps}
                  min={0}
                  max={10000}
                  step={100}
                  style={{ width: '100%' }}
                  onChange={(value) => updateRow(row.id, { teamCapBps: Number(value || 0) })}
                />
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
              </div>
              <div className='grid grid-cols-1 gap-3 lg:grid-cols-3'>
                <InputNumber
                  value={row.teamDecayRatio}
                  min={0}
                  max={1}
                  step={0.1}
                  style={{ width: '100%' }}
                  onChange={(value) => updateRow(row.id, { teamDecayRatio: Number(value || 0) })}
                />
                <InputNumber
                  value={row.teamMaxDepth}
                  min={1}
                  step={1}
                  style={{ width: '100%' }}
                  onChange={(value) => updateRow(row.id, { teamMaxDepth: Number(value || 1) })}
                />
                <div className='flex items-center gap-2'>
                  <Typography.Text>{t('启用')}</Typography.Text>
                  <Switch checked={row.enabled} onChange={(checked) => updateRow(row.id, { enabled: checked })} />
                </div>
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
