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

import React, { useEffect, useState } from 'react';
import { IconPlus } from '@douyinfe/semi-icons';
import {
  Button,
  Card,
  Empty,
  InputNumber,
  Select,
  Space,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../../helpers';
import {
  buildAdminOverrideGroupOptions,
  buildAdminOverrideRows,
  createAdminOverrideDraftRow,
  formatRateBpsPercent,
  normalizeGroupNames,
  percentNumberToRateBps,
} from '../../../../helpers/subscriptionReferral';

const { Text } = Typography;

const buildGroupDefaultRates = (groups = []) =>
  groups.reduce((rateMap, groupItem) => {
    const group = String(groupItem?.group || '').trim();
    if (!group) {
      return rateMap;
    }

    return {
      ...rateMap,
      [group]: Number(
        groupItem?.effective_total_rate_bps ??
          groupItem?.effectiveTotalRateBps ??
          0,
      ),
    };
  }, {});

const SubscriptionReferralOverrideSection = ({ userId }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [savingRowId, setSavingRowId] = useState('');
  const [availableGroups, setAvailableGroups] = useState([]);
  const [groupDefaultRates, setGroupDefaultRates] = useState({});
  const [overrideRows, setOverrideRows] = useState([]);
  const [rowErrors, setRowErrors] = useState({});

  const referralTypeOptions = [{ label: t('订阅返佣'), value: 'subscription' }];

  const clearRowError = (rowId) => {
    setRowErrors((currentErrors) => {
      if (!Object.prototype.hasOwnProperty.call(currentErrors, rowId)) {
        return currentErrors;
      }
      const nextErrors = { ...currentErrors };
      delete nextErrors[rowId];
      return nextErrors;
    });
  };

  const updateRow = (rowId, patch) => {
    setOverrideRows((currentRows) =>
      currentRows.map((row) =>
        row.id === rowId
          ? {
              ...row,
              ...patch,
            }
          : row,
      ),
    );
    clearRowError(rowId);
  };

  const getDefaultRateBpsByGroup = (group, fallbackRateBps = 0) => {
    const normalizedGroup = String(group || '').trim();
    if (!normalizedGroup) {
      return Number(fallbackRateBps || 0);
    }
    if (
      Object.prototype.hasOwnProperty.call(groupDefaultRates, normalizedGroup)
    ) {
      return Number(groupDefaultRates[normalizedGroup] || 0);
    }
    return Number(fallbackRateBps || 0);
  };

  const loadOverrides = async () => {
    if (!userId) return;
    setLoading(true);
    try {
      const [groupRes, userRes] = await Promise.all([
        API.get('/api/group/'),
        API.get(`/api/subscription/admin/referral/users/${userId}`),
      ]);

      if (!groupRes.data?.success) {
        showError(groupRes.data?.message || t('加载失败'));
        return;
      }
      if (!userRes.data?.success) {
        showError(userRes.data?.message || t('加载失败'));
        return;
      }

      const next = userRes.data?.data || {};
      const responseGroups = Array.isArray(next.groups) ? next.groups : [];
      const persistedOverrideRows = buildAdminOverrideRows(
        Array.isArray(next.groups) ? next.groups : [],
      ).filter((row) => row.hasOverride);
      const nextAvailableGroups = normalizeGroupNames([
        ...(Array.isArray(groupRes.data?.data) ? groupRes.data.data : []),
        ...persistedOverrideRows.map((row) => row.group),
      ]);

      setGroupDefaultRates(buildGroupDefaultRates(responseGroups));
      setAvailableGroups(nextAvailableGroups);
      setOverrideRows(persistedOverrideRows);
      setRowErrors({});
    } catch (error) {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadOverrides().then();
  }, [userId]);

  const addOverride = () => {
    const nextRow = createAdminOverrideDraftRow();
    setOverrideRows((currentRows) => [...currentRows, nextRow]);
    clearRowError(nextRow.id);
  };

  const validateRow = (row) => {
    if (!row.type) {
      return t('请选择返佣类型');
    }
    if (!row.group) {
      return t('请选择分组');
    }

    const groupOptions = buildAdminOverrideGroupOptions(
      availableGroups,
      overrideRows,
      row,
    );
    const selectedGroupOption = groupOptions.find(
      (option) => option.value === row.group,
    );
    if (!selectedGroupOption || selectedGroupOption.disabled) {
      return t('该返佣类型和分组组合已存在');
    }

    const percentValue = Number(row.inputPercent);
    if (!Number.isFinite(percentValue)) {
      return t('覆盖总返佣率必须为数字');
    }
    if (percentValue < 0 || percentValue > 100) {
      return t('覆盖总返佣率必须在 0 到 100 之间');
    }
    return '';
  };

  const saveOverride = async (rowId) => {
    const targetRow = overrideRows.find((row) => row.id === rowId);
    if (!targetRow) {
      return;
    }

    const validationError = validateRow(targetRow);
    if (validationError) {
      setRowErrors((currentErrors) => ({
        ...currentErrors,
        [rowId]: validationError,
      }));
      return;
    }

    setSavingRowId(rowId);
    try {
      const res = await API.put(
        `/api/subscription/admin/referral/users/${userId}`,
        {
          group: targetRow.group,
          total_rate_bps: percentNumberToRateBps(targetRow.inputPercent),
        },
      );
      if (res.data?.success) {
        clearRowError(rowId);
        showSuccess(t('保存成功'));
        await loadOverrides();
      } else {
        setRowErrors((currentErrors) => ({
          ...currentErrors,
          [rowId]: res.data?.message || t('保存失败，请重试'),
        }));
      }
    } catch (error) {
      setRowErrors((currentErrors) => ({
        ...currentErrors,
        [rowId]: t('保存失败，请重试'),
      }));
    } finally {
      setSavingRowId('');
    }
  };

  const removeOverride = async (rowId) => {
    const targetRow = overrideRows.find((row) => row.id === rowId);
    if (!targetRow) {
      return;
    }

    if (targetRow.isDraft) {
      setOverrideRows((currentRows) =>
        currentRows.filter((row) => row.id !== rowId),
      );
      clearRowError(rowId);
      return;
    }

    if (!targetRow.hasOverride) {
      return;
    }

    setSavingRowId(rowId);
    try {
      const res = await API.delete(
        `/api/subscription/admin/referral/users/${userId}`,
        {
          params: { group: targetRow.group },
        },
      );
      if (res.data?.success) {
        clearRowError(rowId);
        showSuccess(t('删除成功'));
        await loadOverrides();
      } else {
        setRowErrors((currentErrors) => ({
          ...currentErrors,
          [rowId]: res.data?.message || t('保存失败，请重试'),
        }));
      }
    } catch (error) {
      setRowErrors((currentErrors) => ({
        ...currentErrors,
        [rowId]: t('保存失败，请重试'),
      }));
    } finally {
      setSavingRowId('');
    }
  };

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      <div className='flex flex-col gap-3'>
        <div className='flex items-center justify-between gap-3'>
          <div>
            <Text className='text-lg font-medium'>{t('邀请人返佣覆盖')}</Text>
            <div className='text-xs text-gray-600'>
              {t('暂无覆盖时使用默认返佣规则')}
            </div>
          </div>
          <Button
            icon={<IconPlus />}
            type='primary'
            theme='solid'
            disabled={loading}
            onClick={addOverride}
          >
            {t('新增覆盖')}
          </Button>
        </div>

        {overrideRows.length === 0 ? (
          <div className='rounded-xl border border-dashed border-gray-200 py-8'>
            <Empty
              title={t('暂无覆盖项，未设置时使用默认返佣规则')}
              description=''
            />
          </div>
        ) : (
          <div className='flex flex-col gap-3'>
            {overrideRows.map((row) => {
              const isSaving = savingRowId === row.id;
              const groupOptions = buildAdminOverrideGroupOptions(
                availableGroups,
                overrideRows,
                row,
              );
              return (
                <div
                  key={row.id}
                  className='rounded-xl border border-gray-200 p-4'
                >
                  <div className='grid grid-cols-1 gap-3 lg:grid-cols-2'>
                    <div className='min-w-0'>
                      <Text type='tertiary' className='text-xs block mb-2'>
                        {t('返佣类型')}
                      </Text>
                      <Select
                        value={row.type}
                        optionList={referralTypeOptions}
                        style={{ width: '100%' }}
                        disabled={!row.isDraft || loading || isSaving}
                        onChange={(value) => updateRow(row.id, { type: value })}
                      />
                    </div>
                    <div className='min-w-0'>
                      <Text type='tertiary' className='text-xs block mb-2'>
                        {t('分组')}
                      </Text>
                      <Select
                        value={row.group || undefined}
                        optionList={groupOptions}
                        style={{ width: '100%' }}
                        disabled={!row.isDraft || loading || isSaving}
                        onChange={(value) =>
                          updateRow(row.id, {
                            group: value,
                            effectiveTotalRateBps:
                              getDefaultRateBpsByGroup(value),
                          })
                        }
                      />
                    </div>
                    <div className='min-w-0 lg:col-span-2'>
                      <Text type='tertiary' className='text-xs block mb-2'>
                        {t('覆盖总返佣率')}
                      </Text>
                      <InputNumber
                        value={row.inputPercent}
                        min={0}
                        max={100}
                        step={0.01}
                        precision={2}
                        suffix='%'
                        style={{ width: '100%' }}
                        disabled={loading || isSaving}
                        onChange={(value) =>
                          updateRow(row.id, { inputPercent: value })
                        }
                      />
                    </div>
                  </div>

                  <div className='mt-2 text-xs text-gray-500'>
                    {`${t('当前默认总返佣率')} ${formatRateBpsPercent(
                      getDefaultRateBpsByGroup(
                        row.group,
                        row.effectiveTotalRateBps,
                      ),
                    )}`}
                  </div>
                  {rowErrors[row.id] ? (
                    <div className='mt-1 text-xs text-red-500'>
                      {rowErrors[row.id]}
                    </div>
                  ) : null}

                  <Space className='mt-3'>
                    <Button
                      type='primary'
                      theme='solid'
                      loading={isSaving}
                      disabled={loading}
                      onClick={() => saveOverride(row.id)}
                    >
                      {t('保存')}
                    </Button>
                    <Button
                      theme='light'
                      disabled={
                        (!row.isDraft && !row.hasOverride) ||
                        loading ||
                        isSaving
                      }
                      onClick={() => removeOverride(row.id)}
                    >
                      {row.isDraft ? t('取消') : t('删除')}
                    </Button>
                  </Space>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </Card>
  );
};

export default SubscriptionReferralOverrideSection;
