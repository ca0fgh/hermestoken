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

import React, { useEffect, useRef, useState } from 'react';
import { Button, Col, Form, Row, Spin } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, showWarning } from '../../../helpers';
import {
  buildAdminReferralRows,
  normalizeGroupRateMap,
  parseAdminReferralSettings,
  percentNumberToRateBps,
} from '../../../helpers/subscriptionReferral';

export default function SettingsSubscriptionReferral() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const [groupNames, setGroupNames] = useState([]);
  const [groupRows, setGroupRows] = useState([]);
  const [snapshot, setSnapshot] = useState({ enabled: false, groupRates: {} });
  const refForm = useRef(null);

  const formValues = groupRows.reduce(
    (values, row) => ({
      ...values,
      [`SubscriptionReferralGroupEnabled_${row.group}`]: row.enabled,
      [`SubscriptionReferralGroupRate_${row.group}`]: row.totalRatePercent,
    }),
    {
      SubscriptionReferralEnabled: enabled,
    },
  );

  const applySettings = (payload, nextGroupNames = groupNames) => {
    const nextSettings = parseAdminReferralSettings(payload);
    const nextRows = buildAdminReferralRows(
      nextGroupNames,
      nextSettings.groupRates,
    );

    setEnabled(nextSettings.enabled);
    setGroupRows(nextRows);
    setGroupNames(nextRows.map((row) => row.group));
    setSnapshot({
      enabled: nextSettings.enabled,
      groupRates: normalizeGroupRateMap(nextSettings.groupRates),
    });
  };

  useEffect(() => {
    // Semi Form field components read from form store first, so we must keep
    // the form API in sync with the externally loaded settings.
    refForm.current?.setValues(formValues);
  }, [enabled, groupRows]);

  const loadSettings = async () => {
    setLoading(true);
    try {
      const settingsRes = await API.get('/api/subscription/admin/referral/settings');
      if (settingsRes.data?.success) {
        const settingsPayload = settingsRes.data?.data || {};
        const fallbackSettings = parseAdminReferralSettings(settingsPayload);
        let nextGroupNames = Object.keys(fallbackSettings.groupRates || {});

        try {
          const groupsRes = await API.get('/api/group/');
          if (Array.isArray(groupsRes.data?.data) && groupsRes.data.data.length > 0) {
            nextGroupNames = groupsRes.data.data;
          }
        } catch (error) {
          // Keep rendering from settings payload when the group catalog is unavailable.
        }

        applySettings(settingsPayload, nextGroupNames);
      } else {
        showError(settingsRes.data?.message || t('加载失败'));
      }
    } catch (error) {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadSettings().then();
  }, []);

  const updateGroupRow = (group, updater) => {
    setGroupRows((currentRows) =>
      currentRows.map((row) => {
        if (row.group !== group) {
          return row;
        }
        return updater(row);
      }),
    );
  };

  const onSubmit = async () => {
    const nextGroupRates = normalizeGroupRateMap(
      groupRows.reduce(
        (rates, row) => ({
          ...rates,
          [row.group]: row.enabled ? percentNumberToRateBps(row.totalRatePercent) : 0,
        }),
        {},
      ),
    );
    const groupsToCompare = new Set([
      ...groupNames,
      ...Object.keys(snapshot.groupRates || {}),
      ...Object.keys(nextGroupRates),
    ]);
    const hasChanged =
      enabled !== snapshot.enabled ||
      Array.from(groupsToCompare).some(
        (group) => (snapshot.groupRates?.[group] || 0) !== (nextGroupRates[group] || 0),
      );

    if (!hasChanged) {
      return showWarning(t('你似乎并没有修改什么'));
    }

    setLoading(true);
    try {
      const res = await API.put('/api/subscription/admin/referral/settings', {
        enabled,
        group_rates: nextGroupRates,
      });
      if (res.data?.success) {
        applySettings(res.data?.data || {}, groupNames);
        showSuccess(t('保存成功'));
      } else {
        showError(res.data?.message || t('保存失败，请重试'));
      }
    } catch (error) {
      showError(t('保存失败，请重试'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Spin spinning={loading}>
      <Form
        values={formValues}
        getFormApi={(formApi) => {
          refForm.current = formApi;
        }}
        style={{ marginBottom: 15 }}
      >
        <Form.Section text={t('订阅返佣设置')}>
          <Row gutter={16}>
            <Col xs={24} sm={12} md={10}>
              <Form.Switch
                field='SubscriptionReferralEnabled'
                label={t('启用订阅返佣')}
                checked={enabled}
                onChange={(value) => setEnabled(value)}
              />
            </Col>
          </Row>
          {groupRows.map((row) => (
            <Row gutter={16} key={row.group}>
              <Col xs={24} sm={12} md={10}>
                <Form.Switch
                  field={`SubscriptionReferralGroupEnabled_${row.group}`}
                  label={`${row.group} ${t('启用返佣')}`}
                  checked={row.enabled}
                  onChange={(value) => {
                    updateGroupRow(row.group, (currentRow) => ({
                      ...currentRow,
                      enabled: value,
                      totalRateBps: value ? currentRow.totalRateBps : 0,
                      totalRatePercent: value ? currentRow.totalRatePercent : 0,
                    }));
                  }}
                />
              </Col>
              <Col xs={24} sm={12} md={10}>
                <Form.InputNumber
                  field={`SubscriptionReferralGroupRate_${row.group}`}
                  label={`${row.group} ${t('总返佣率')}`}
                  value={row.totalRatePercent}
                  min={0}
                  max={100}
                  step={0.01}
                  precision={2}
                  suffix='%'
                  disabled={!row.enabled}
                  onChange={(value) => {
                    const totalRatePercent = Number(value || 0);
                    updateGroupRow(row.group, (currentRow) => ({
                      ...currentRow,
                      totalRateBps: percentNumberToRateBps(totalRatePercent),
                      totalRatePercent,
                    }));
                  }}
                />
              </Col>
            </Row>
          ))}
          <Row>
            <Button size='default' onClick={onSubmit}>
              {t('保存订阅返佣设置')}
            </Button>
          </Row>
        </Form.Section>
      </Form>
    </Spin>
  );
}
