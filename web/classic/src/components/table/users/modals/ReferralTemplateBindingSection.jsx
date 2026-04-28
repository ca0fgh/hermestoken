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

For commercial licensing, please contact support@quantumnous.com
*/
import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../../helpers';
import { Button, Card, Empty, Select, Space, Typography } from '@douyinfe/semi-ui';
import { formatReferralTemplateOptionLabel } from '../../../../helpers/referralLabels';
import { normalizeReferralTemplateItems } from '../../../../helpers/referralTemplate';

const normalizeBindingIds = (bindingIds = []) =>
  [...new Set((Array.isArray(bindingIds) ? bindingIds : []).map((bindingId) => Number(bindingId || 0)).filter((bindingId) => bindingId > 0))].sort(
    (left, right) => left - right,
  );

const normalizeBindingRows = (items = []) => {
  const sourceItems = Array.isArray(items) ? items : [];
  const normalizedItems = normalizeReferralTemplateItems(sourceItems);
  return normalizedItems.map((item, index) => {
    const bindingIds = normalizeBindingIds(sourceItems[index]?.binding_ids);
    return {
      id: `binding-${bindingIds.join('-') || item.bundleKey || item.id || index}`,
      bindingIds,
      referralType: String(item.referralType || 'subscription_referral').trim(),
      templateId: Number(item.id || 0),
      isDraft: false,
    };
  });
};

const createDraftBinding = (templates = []) => {
  const firstTemplate = Array.isArray(templates) && templates.length > 0 ? templates[0] : null;
  return {
    id: `draft-${Date.now()}-${Math.random()}`,
    bindingIds: [],
    referralType: String(firstTemplate?.referralType || 'subscription_referral').trim(),
    templateId: Number(firstTemplate?.id || 0),
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
        label: formatReferralTemplateOptionLabel(template, t, {
          includeGroupSuffixWhenNamed: true,
        }),
        value: template.id,
      })),
    [t, templates],
  );

  const load = async () => {
    if (!userId) {
      return;
    }
    setLoading(true);
    try {
      const [templateRes, bindingRes] = await Promise.all([
        API.get('/api/referral/templates', {
          params: { referral_type: 'subscription_referral', view: 'bundle' },
        }),
        API.get(`/api/referral/bindings/users/${userId}`, {
          params: { referral_type: 'subscription_referral', view: 'bundle' },
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

      const nextTemplates = normalizeReferralTemplateItems(templateRes.data?.data?.items || []);
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
        template_id: row.templateId,
        replace_binding_ids: row.bindingIds,
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
              {t('给当前用户选择生效模板。分组和默认返给被邀请人比例都直接跟模板走。')}
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
              <div className='grid grid-cols-1 gap-3'>
                <div>
                  <Typography.Text type='tertiary' className='text-xs block mb-2'>
                    {t('模板')}
                  </Typography.Text>
                  <Select
                    value={row.templateId || undefined}
                    style={{ width: '100%' }}
                    optionList={templateOptions}
                    onChange={(value) => {
                      updateRow(row.id, {
                        templateId: Number(value || 0),
                      });
                    }}
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
