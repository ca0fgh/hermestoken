import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../../helpers';
import { Button, Card, Empty, InputNumber, Select, Space, Typography } from '@douyinfe/semi-ui';

const ReferralTemplateBindingSection = ({ userId }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [templates, setTemplates] = useState([]);
  const [bindings, setBindings] = useState([]);
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

      if (templateRes.data?.success) {
        setTemplates(templateRes.data?.data?.items || []);
      } else {
        showError(templateRes.data?.message || t('加载失败'));
      }
      if (bindingRes.data?.success) {
        setBindings(bindingRes.data?.data?.items || []);
      } else {
        showError(bindingRes.data?.message || t('加载失败'));
      }
    } catch (error) {
      showError(error?.message || t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, [userId]);

  const saveBinding = async (binding) => {
    const key = `${binding.binding?.group || binding.group || 'draft'}`;
    setSavingKey(key);
    try {
      const res = await API.put(`/api/referral/bindings/users/${userId}`, {
        referral_type: binding.binding?.referral_type || 'subscription_referral',
        group: binding.binding?.group || binding.group,
        template_id: binding.binding?.template_id || binding.template?.id || binding.template_id,
        invitee_share_override_bps: binding.binding?.invitee_share_override_bps ?? null,
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
        <div>
          <Typography.Text className='text-lg font-medium'>
            {t('返佣模板绑定')}
          </Typography.Text>
          <div className='text-xs text-gray-600'>
            {t('按返佣类型与分组管理用户当前绑定的模板和默认分账比例。')}
          </div>
        </div>
        {bindings.length === 0 ? (
          <Empty description={t('暂无返佣模板绑定')} />
        ) : (
          bindings.map((view) => (
            <div
              key={`${view.binding?.referral_type}-${view.binding?.group}`}
              className='rounded-xl border border-gray-200 p-4'
            >
              <div className='grid grid-cols-1 gap-3 lg:grid-cols-3'>
                <div>
                  <Typography.Text type='tertiary' className='text-xs block mb-2'>
                    {t('分组')}
                  </Typography.Text>
                  <Typography.Text>{view.binding?.group}</Typography.Text>
                </div>
                <div>
                  <Typography.Text type='tertiary' className='text-xs block mb-2'>
                    {t('模板')}
                  </Typography.Text>
                  <Select
                    value={view.binding?.template_id}
                    style={{ width: '100%' }}
                    optionList={templateOptions.filter(
                      (option) => option.group === view.binding?.group,
                    )}
                    onChange={(value) => {
                      view.binding.template_id = value;
                    }}
                  />
                </div>
                <div>
                  <Typography.Text type='tertiary' className='text-xs block mb-2'>
                    {t('默认分账比例')}
                  </Typography.Text>
                  <InputNumber
                    value={view.binding?.invitee_share_override_bps ?? 0}
                    min={0}
                    max={10000}
                    step={100}
                    style={{ width: '100%' }}
                    onChange={(value) => {
                      view.binding.invitee_share_override_bps =
                        value === 0 || value === null ? null : Number(value);
                    }}
                  />
                </div>
              </div>
              <Space className='mt-3'>
                <Button
                  type='primary'
                  loading={savingKey === view.binding?.group}
                  onClick={() => saveBinding(view)}
                >
                  {t('保存')}
                </Button>
              </Space>
            </div>
          ))
        )}
      </div>
    </Card>
  );
};

export default ReferralTemplateBindingSection;
