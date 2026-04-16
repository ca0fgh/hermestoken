import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../../helpers';
import { Button, Card, Empty, InputNumber, Select, Space, Typography } from '@douyinfe/semi-ui';

const normalizeBindingRows = (items = []) =>
  (Array.isArray(items) ? items : []).map((view) => ({
    id: `${view.binding?.referral_type || 'subscription_referral'}:${view.binding?.group || ''}`,
    bindingId: Number(view.binding?.id || 0),
    referralType: String(view.binding?.referral_type || 'subscription_referral').trim(),
    group: String(view.binding?.group || '').trim(),
    templateId: Number(view.binding?.template_id || 0),
    inviteeShareOverrideBps:
      view.binding?.invitee_share_override_bps === null ||
      view.binding?.invitee_share_override_bps === undefined
        ? null
        : Number(view.binding?.invitee_share_override_bps || 0),
    isDraft: false,
  }));

const createDraftBinding = (templates = []) => {
  const firstTemplate = Array.isArray(templates) && templates.length > 0 ? templates[0] : null;
  return {
    id: `draft-${Date.now()}-${Math.random()}`,
    bindingId: 0,
    referralType: 'subscription_referral',
    group: String(firstTemplate?.group || '').trim(),
    templateId: Number(firstTemplate?.id || 0),
    inviteeShareOverrideBps: null,
    isDraft: true,
  };
};

const ReferralTemplateBindingSection = ({ userId }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [templates, setTemplates] = useState([]);
  const [bindingRows, setBindingRows] = useState([]);
  const [savingKey, setSavingKey] = useState('');

  const templateOptions = useMemo(
    () =>
      templates.map((template) => ({
        label: `${template.name} · ${template.group} · ${template.level_type}`,
        value: template.id,
        group: template.group,
      })),
    [templates],
  );

  const load = async () => {
    if (!userId) {
      return;
    }
    setLoading(true);
    try {
      const [templateRes, bindingRes] = await Promise.all([
        API.get('/api/referral/templates', {
          params: { referral_type: 'subscription_referral' },
        }),
        API.get(`/api/referral/bindings/users/${userId}`, {
          params: { referral_type: 'subscription_referral' },
        }),
      ]);

      if (!templateRes.data?.success) {
        showError(templateRes.data?.message || t('加载失败'));
        return;
      }
      if (!bindingRes.data?.success) {
        showError(bindingRes.data?.message || t('加载失败'));
        return;
      }

      const nextTemplates = templateRes.data?.data?.items || [];
      setTemplates(nextTemplates);
      setBindingRows(normalizeBindingRows(bindingRes.data?.data?.items));
    } catch (error) {
      showError(error?.message || t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, [userId]);

  const updateRow = (id, patch) => {
    setBindingRows((currentRows) =>
      currentRows.map((row) => (row.id === id ? { ...row, ...patch } : row)),
    );
  };

  const addDraft = () => {
    setBindingRows((currentRows) => [...currentRows, createDraftBinding(templates)]);
  };

  const removeDraft = (id) => {
    setBindingRows((currentRows) => currentRows.filter((row) => row.id !== id));
  };

  const saveBinding = async (row) => {
    setSavingKey(String(row.id));
    try {
      const res = await API.put(`/api/referral/bindings/users/${userId}`, {
        referral_type: row.referralType,
        group: row.group,
        template_id: row.templateId,
        invitee_share_override_bps: row.inviteeShareOverrideBps,
      });
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

  if (!userId) {
    return null;
  }

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      <div className='space-y-3'>
        <div className='flex items-center justify-between gap-3'>
          <div>
            <Typography.Text className='text-lg font-medium'>
              {t('返佣模板绑定')}
            </Typography.Text>
            <div className='text-xs text-gray-600'>
              {t('按返佣类型与分组管理用户当前绑定的模板和默认分账比例。')}
            </div>
          </div>
          <Button type='primary' onClick={addDraft} disabled={templates.length === 0}>
            {t('新增绑定')}
          </Button>
        </div>
        {bindingRows.length === 0 ? (
          <Empty description={t('暂无返佣模板绑定')} />
        ) : (
          bindingRows.map((row) => (
            <div key={row.id} className='rounded-xl border border-gray-200 p-4'>
              <div className='grid grid-cols-1 gap-3 lg:grid-cols-3'>
                <div>
                  <Typography.Text type='tertiary' className='text-xs block mb-2'>
                    {t('分组')}
                  </Typography.Text>
                  <Typography.Text>{row.group || '-'}</Typography.Text>
                </div>
                <div>
                  <Typography.Text type='tertiary' className='text-xs block mb-2'>
                    {t('模板')}
                  </Typography.Text>
                  <Select
                    value={row.templateId || undefined}
                    style={{ width: '100%' }}
                    optionList={templateOptions}
                    onChange={(value) => {
                      const selectedTemplate = templates.find((template) => Number(template.id) === Number(value));
                      updateRow(row.id, {
                        templateId: Number(value || 0),
                        group: String(selectedTemplate?.group || '').trim(),
                      });
                    }}
                  />
                </div>
                <div>
                  <Typography.Text type='tertiary' className='text-xs block mb-2'>
                    {t('默认分账比例')}
                  </Typography.Text>
                  <InputNumber
                    value={row.inviteeShareOverrideBps ?? 0}
                    min={0}
                    max={10000}
                    step={100}
                    style={{ width: '100%' }}
                    onChange={(value) =>
                      updateRow(row.id, {
                        inviteeShareOverrideBps:
                          value === null || Number(value) === 0 ? null : Number(value),
                      })
                    }
                  />
                </div>
              </div>
              <Space className='mt-3'>
                <Button
                  type='primary'
                  loading={savingKey === String(row.id)}
                  onClick={() => saveBinding(row)}
                >
                  {t('保存')}
                </Button>
                {row.isDraft ? (
                  <Button theme='borderless' type='danger' onClick={() => removeDraft(row.id)}>
                    {t('删除')}
                  </Button>
                ) : null}
              </Space>
            </div>
          ))
        )}
      </div>
    </Card>
  );
};

export default ReferralTemplateBindingSection;
