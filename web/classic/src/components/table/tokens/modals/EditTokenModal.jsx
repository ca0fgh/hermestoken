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

import React, { useEffect, useState, useContext, useRef } from 'react';
import {
  API,
  processGroupsData,
  showError,
  showSuccess,
  timestamp2string,
  renderGroupOption,
  renderQuota,
  getCurrencyConfig,
  getModelCategories,
  selectFilter,
} from '../../../../helpers';
import {
  quotaToDisplayAmount,
  displayAmountToQuota,
} from '../../../../helpers/quota';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import {
  Button,
  SideSheet,
  Space,
  Spin,
  Typography,
  Card,
  Tag,
  Avatar,
  Form,
  Col,
  Row,
  Switch,
} from '@douyinfe/semi-ui';
import {
  IconCreditCard,
  IconLink,
  IconSave,
  IconClose,
  IconKey,
  IconChevronUp,
  IconChevronDown,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { StatusContext } from '../../../../context/Status';

const { Text, Title } = Typography;
const DEFAULT_TOKEN_GROUP = '';
const MARKETPLACE_ROUTE_ORDER_VALUES = ['fixed_order', 'group', 'pool'];
const DEFAULT_MARKETPLACE_ROUTE_ORDER = ['fixed_order', 'group', 'pool'];
const DEFAULT_MARKETPLACE_ROUTE_ENABLED = ['fixed_order', 'group', 'pool'];
const MARKETPLACE_ROUTE_ORDER_LABELS = {
  fixed_order: '市场买断订单',
  group: '普通分组订单',
  pool: '订单池',
};
const MARKETPLACE_ROUTE_ALIASES = {
  fixed_order: 'fixed_order',
  marketplace_fixed_order: 'fixed_order',
  fixed: 'fixed_order',
  order: 'fixed_order',
  group: 'group',
  normal_group: 'group',
  ordinary_group: 'group',
  channel: 'group',
  pool: 'pool',
  marketplace_pool: 'pool',
  order_pool: 'pool',
};

const normalizeMarketplaceRouteOrder = (value) => {
  const rawRoutes = Array.isArray(value)
    ? value
    : typeof value === 'string'
      ? value.split(',')
      : [];
  const seen = new Set();
  const normalized = [];

  rawRoutes.forEach((rawRoute) => {
    const route = MARKETPLACE_ROUTE_ALIASES[String(rawRoute).trim()];
    if (!route || seen.has(route)) return;
    seen.add(route);
    normalized.push(route);
  });

  DEFAULT_MARKETPLACE_ROUTE_ORDER.forEach((route) => {
    if (seen.has(route)) return;
    normalized.push(route);
  });

  return normalized;
};

const normalizeMarketplaceRouteEnabled = (value) => {
  if (value == null) return [...DEFAULT_MARKETPLACE_ROUTE_ENABLED];

  const rawRoutes = Array.isArray(value)
    ? value
    : typeof value === 'string'
      ? value.split(',')
      : [];
  const seen = new Set();
  const normalized = [];

  rawRoutes.forEach((rawRoute) => {
    const route = MARKETPLACE_ROUTE_ALIASES[String(rawRoute).trim()];
    if (!route || seen.has(route)) return;
    seen.add(route);
    normalized.push(route);
  });

  return normalized;
};

const EditTokenModal = (props) => {
  const { t } = useTranslation();
  const [statusState, statusDispatch] = useContext(StatusContext);
  const [loading, setLoading] = useState(false);
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);
  const [models, setModels] = useState([]);
  const [groups, setGroups] = useState([]);
  const [marketplaceFixedOrders, setMarketplaceFixedOrders] = useState([]);
  const [showQuotaInput, setShowQuotaInput] = useState(false);
  const [marketplaceRouteOrderValue, setMarketplaceRouteOrderValue] = useState([
    ...DEFAULT_MARKETPLACE_ROUTE_ORDER,
  ]);
  const [marketplaceRouteEnabledValue, setMarketplaceRouteEnabledValue] =
    useState([...DEFAULT_MARKETPLACE_ROUTE_ENABLED]);
  const isEdit = props.editingToken.id !== undefined;
  const normalizeTokenGroupInputs = (inputs) => {
    const group = inputs.group || DEFAULT_TOKEN_GROUP;
    return {
      ...inputs,
      group,
      cross_group_retry:
        group === DEFAULT_TOKEN_GROUP ? !!inputs.cross_group_retry : false,
    };
  };
  const applyMarketplaceRouteOrderToForm = (routeOrder) => {
    const normalized = normalizeMarketplaceRouteOrder(routeOrder);
    setMarketplaceRouteOrderValue(normalized);
    formApiRef.current?.setValue('marketplace_route_order', normalized);
    normalized.forEach((route, index) => {
      formApiRef.current?.setValue(`marketplace_route_order_${index}`, route);
    });
  };
  const moveMarketplaceRouteOrderItem = (current, index, direction) => {
    const next = normalizeMarketplaceRouteOrder(current);
    const targetIndex = index + direction;
    if (targetIndex < 0 || targetIndex >= next.length) return next;
    const moved = [...next];
    [moved[index], moved[targetIndex]] = [moved[targetIndex], moved[index]];
    return normalizeMarketplaceRouteOrder(moved);
  };
  const handleMarketplaceRouteOrderMove = (index, direction) => {
    applyMarketplaceRouteOrderToForm(
      moveMarketplaceRouteOrderItem(
        marketplaceRouteOrderValue,
        index,
        direction,
      ),
    );
  };
  const applyMarketplaceRouteEnabledToForm = (routes) => {
    const normalized = normalizeMarketplaceRouteEnabled(routes);
    setMarketplaceRouteEnabledValue(normalized);
    formApiRef.current?.setValue('marketplace_route_enabled', normalized);
    return normalized;
  };
  const handleMarketplaceRouteEnabledChange = (route, enabled) => {
    const current = new Set(marketplaceRouteEnabledValue);
    if (enabled) {
      current.add(route);
    } else {
      current.delete(route);
    }
    applyMarketplaceRouteEnabledToForm(
      MARKETPLACE_ROUTE_ORDER_VALUES.filter((item) => current.has(item)),
    );
  };
  const resetMarketplaceRouteFormValues = () => {
    applyMarketplaceRouteOrderToForm(DEFAULT_MARKETPLACE_ROUTE_ORDER);
    applyMarketplaceRouteEnabledToForm(DEFAULT_MARKETPLACE_ROUTE_ENABLED);
  };

  const getInitValues = () => ({
    name: '',
    remain_quota: 0,
    remain_amount: 0,
    expired_time: -1,
    unlimited_quota: true,
    model_limits_enabled: false,
    model_limits: [],
    allow_ips: '',
    group: DEFAULT_TOKEN_GROUP,
    cross_group_retry: false,
    marketplace_fixed_order_id: 0,
    marketplace_fixed_order_ids: [],
    marketplace_route_order: [...DEFAULT_MARKETPLACE_ROUTE_ORDER],
    marketplace_route_enabled: [...DEFAULT_MARKETPLACE_ROUTE_ENABLED],
    marketplace_route_order_0: DEFAULT_MARKETPLACE_ROUTE_ORDER[0],
    marketplace_route_order_1: DEFAULT_MARKETPLACE_ROUTE_ORDER[1],
    marketplace_route_order_2: DEFAULT_MARKETPLACE_ROUTE_ORDER[2],
    tokenCount: 1,
  });

  const handleCancel = () => {
    props.handleClose();
  };

  const setExpiredTime = (month, day, hour, minute) => {
    let now = new Date();
    let timestamp = now.getTime() / 1000;
    let seconds = month * 30 * 24 * 60 * 60;
    seconds += day * 24 * 60 * 60;
    seconds += hour * 60 * 60;
    seconds += minute * 60;
    if (!formApiRef.current) return;
    if (seconds !== 0) {
      timestamp += seconds;
      formApiRef.current.setValue('expired_time', timestamp2string(timestamp));
    } else {
      formApiRef.current.setValue('expired_time', -1);
    }
  };

  const normalizeMarketplaceFixedOrderInputs = (inputs) => {
    const fixedOrderIds = Array.isArray(inputs.marketplace_fixed_order_ids)
      ? inputs.marketplace_fixed_order_ids
          .map((id) => Number(id))
          .filter((id) => Number.isFinite(id) && id > 0)
      : inputs.marketplace_fixed_order_id
        ? [Number(inputs.marketplace_fixed_order_id)]
        : [];
    return {
      ...inputs,
      marketplace_fixed_order_id: fixedOrderIds[0] || 0,
      marketplace_fixed_order_ids: Array.from(new Set(fixedOrderIds)),
    };
  };
  const normalizeMarketplaceRouteOrderInputs = (inputs) => {
    const {
      marketplace_route_order_0,
      marketplace_route_order_1,
      marketplace_route_order_2,
      ...rest
    } = inputs;
    return {
      ...rest,
      marketplace_route_order: normalizeMarketplaceRouteOrder([
        marketplace_route_order_0,
        marketplace_route_order_1,
        marketplace_route_order_2,
        ...(Array.isArray(inputs.marketplace_route_order)
          ? inputs.marketplace_route_order
          : []),
      ]),
      marketplace_route_enabled: normalizeMarketplaceRouteEnabled(
        inputs.marketplace_route_enabled,
      ),
    };
  };

  const loadModels = async () => {
    let res = await API.get(`/api/user/models`);
    const { success, message, data } = res.data;
    if (success) {
      const categories = getModelCategories(t);
      let localModelOptions = data.map((model) => {
        let icon = null;
        for (const [key, category] of Object.entries(categories)) {
          if (key !== 'all' && category.filter({ model_name: model })) {
            icon = category.icon;
            break;
          }
        }
        return {
          label: (
            <span className='flex items-center gap-1'>
              {icon}
              {model}
            </span>
          ),
          value: model,
        };
      });
      setModels(localModelOptions);
    } else {
      showError(t(message));
    }
  };

  const loadGroups = async () => {
    let res = await API.get(`/api/token/groups`);
    const { success, message, data } = res.data;
    if (success) {
      const localGroupOptions =
        Object.keys(data || {}).length === 0 ? [] : processGroupsData(data);
      if (statusState?.status?.default_use_auto_group) {
        if (localGroupOptions.some((group) => group.value === 'auto')) {
          localGroupOptions.sort((a, b) => (a.value === 'auto' ? -1 : 1));
        }
      }
      setGroups(localGroupOptions);
      const currentGroup = formApiRef.current?.getValue?.('group');
      const groupStillSelectable = localGroupOptions.some(
        (group) => group.value === currentGroup,
      );
      if (currentGroup && !groupStillSelectable) {
        formApiRef.current?.setValue('group', DEFAULT_TOKEN_GROUP);
      }
    } else {
      showError(t(message));
    }
  };

  const loadMarketplaceFixedOrders = async () => {
    try {
      const res = await API.get(`/api/marketplace/fixed-orders`, {
        params: { p: 1, page_size: 100 },
      });
      const { success, data } = res.data;
      setMarketplaceFixedOrders(success ? data?.items || [] : []);
    } catch {
      setMarketplaceFixedOrders([]);
    }
  };

  const loadToken = async () => {
    setLoading(true);
    let res = await API.get(`/api/token/${props.editingToken.id}`);
    const { success, message, data } = res.data;
    if (success) {
      if (data.expired_time !== -1) {
        data.expired_time = timestamp2string(data.expired_time);
      }
      if (data.model_limits !== '') {
        data.model_limits = data.model_limits.split(',');
      } else {
        data.model_limits = [];
      }
      data.remain_amount = Number(
        quotaToDisplayAmount(data.remain_quota || 0).toFixed(6),
      );
      const fixedOrderIds = Array.isArray(data.marketplace_fixed_order_ids)
        ? data.marketplace_fixed_order_ids
        : typeof data.marketplace_fixed_order_ids === 'string'
          ? data.marketplace_fixed_order_ids
              .split(',')
              .map((id) => Number(id))
              .filter((id) => Number.isFinite(id) && id > 0)
          : [];
      data.marketplace_fixed_order_ids =
        fixedOrderIds.length > 0
          ? fixedOrderIds
          : data.marketplace_fixed_order_id
            ? [data.marketplace_fixed_order_id]
            : [];
      data.marketplace_fixed_order_id =
        data.marketplace_fixed_order_ids[0] || 0;
      data.marketplace_route_order = normalizeMarketplaceRouteOrder(
        data.marketplace_route_order,
      );
      data.marketplace_route_enabled = normalizeMarketplaceRouteEnabled(
        data.marketplace_route_enabled,
      );
      setMarketplaceRouteOrderValue(data.marketplace_route_order);
      setMarketplaceRouteEnabledValue(data.marketplace_route_enabled);
      data.marketplace_route_order_0 = data.marketplace_route_order[0];
      data.marketplace_route_order_1 = data.marketplace_route_order[1];
      data.marketplace_route_order_2 = data.marketplace_route_order[2];
      data.group = data.group || DEFAULT_TOKEN_GROUP;
      if (formApiRef.current) {
        formApiRef.current.setValues({ ...getInitValues(), ...data });
      }
    } else {
      showError(message);
    }
    setLoading(false);
  };

  useEffect(() => {
    if (formApiRef.current) {
      if (!isEdit) {
        formApiRef.current.setValues(getInitValues());
        resetMarketplaceRouteFormValues();
      }
    }
    loadModels();
    loadGroups();
    loadMarketplaceFixedOrders();
  }, [props.editingToken.id]);

  useEffect(() => {
    if (props.visiable) {
      if (isEdit) {
        loadToken();
      } else {
        formApiRef.current?.setValues(getInitValues());
        resetMarketplaceRouteFormValues();
      }
    } else {
      formApiRef.current?.reset();
      resetMarketplaceRouteFormValues();
    }
  }, [props.visiable, props.editingToken.id]);

  const generateRandomSuffix = () => {
    const characters =
      'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
    let result = '';
    for (let i = 0; i < 6; i++) {
      result += characters.charAt(
        Math.floor(Math.random() * characters.length),
      );
    }
    return result;
  };

  const submit = async (values) => {
    setLoading(true);
    if (isEdit) {
      let { tokenCount: _tc, ...localInputs } = values;
      localInputs.remain_quota = localInputs.unlimited_quota
        ? 0
        : displayAmountToQuota(localInputs.remain_amount);
      if (!localInputs.unlimited_quota && localInputs.remain_quota <= 0) {
        showError(t('请输入金额'));
        setLoading(false);
        return;
      }
      if (localInputs.expired_time !== -1) {
        let time = Date.parse(localInputs.expired_time);
        if (isNaN(time)) {
          showError(t('过期时间格式错误！'));
          setLoading(false);
          return;
        }
        localInputs.expired_time = Math.ceil(time / 1000);
      }
      localInputs = normalizeTokenGroupInputs(localInputs);
      localInputs = normalizeMarketplaceFixedOrderInputs(localInputs);
      localInputs = normalizeMarketplaceRouteOrderInputs({
        ...localInputs,
        marketplace_route_order: marketplaceRouteOrderValue,
        marketplace_route_enabled: marketplaceRouteEnabledValue,
      });
      localInputs.model_limits = localInputs.model_limits.join(',');
      localInputs.model_limits_enabled = localInputs.model_limits.length > 0;
      let res = await API.put(`/api/token/`, {
        ...localInputs,
        id: parseInt(props.editingToken.id),
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('令牌更新成功！'));
        props.refresh();
        props.handleClose();
      } else {
        showError(t(message));
      }
    } else {
      const count = parseInt(values.tokenCount, 10) || 1;
      let successCount = 0;
      for (let i = 0; i < count; i++) {
        let { tokenCount: _tc, ...localInputs } = values;
        const baseName =
          values.name.trim() === '' ? 'default' : values.name.trim();
        if (i !== 0 || values.name.trim() === '') {
          localInputs.name = `${baseName}-${generateRandomSuffix()}`;
        } else {
          localInputs.name = baseName;
        }
        localInputs.remain_quota = localInputs.unlimited_quota
          ? 0
          : displayAmountToQuota(localInputs.remain_amount);
        if (!localInputs.unlimited_quota && localInputs.remain_quota <= 0) {
          showError(t('请输入金额'));
          setLoading(false);
          break;
        }

        if (localInputs.expired_time !== -1) {
          let time = Date.parse(localInputs.expired_time);
          if (isNaN(time)) {
            showError(t('过期时间格式错误！'));
            setLoading(false);
            break;
          }
          localInputs.expired_time = Math.ceil(time / 1000);
        }
        localInputs = normalizeTokenGroupInputs(localInputs);
        localInputs = normalizeMarketplaceFixedOrderInputs(localInputs);
        localInputs = normalizeMarketplaceRouteOrderInputs({
          ...localInputs,
          marketplace_route_order: marketplaceRouteOrderValue,
          marketplace_route_enabled: marketplaceRouteEnabledValue,
        });
        localInputs.model_limits = localInputs.model_limits.join(',');
        localInputs.model_limits_enabled = localInputs.model_limits.length > 0;
        let res = await API.post(`/api/token/`, localInputs);
        const { success, message } = res.data;
        if (success) {
          successCount++;
        } else {
          showError(t(message));
          break;
        }
      }
      if (successCount > 0) {
        showSuccess(t('令牌创建成功，请在列表页面点击复制获取令牌！'));
        props.refresh();
        props.handleClose();
      }
    }
    setLoading(false);
    formApiRef.current?.setValues(getInitValues());
    resetMarketplaceRouteFormValues();
  };

  const marketplaceFixedOrderOptions = [
    ...marketplaceFixedOrders.map((order) => ({
      label: `#${order.id} · ${t('托管Key')} #${order.credential_id} · ${t(
        order.status,
      )} · ${renderQuota(order.remaining_quota || 0)}`,
      value: order.id,
    })),
  ];
  return (
    <SideSheet
      placement={isEdit ? 'right' : 'left'}
      title={
        <Space>
          {isEdit ? (
            <Tag color='blue' shape='circle'>
              {t('更新')}
            </Tag>
          ) : (
            <Tag color='green' shape='circle'>
              {t('新建')}
            </Tag>
          )}
          <Title heading={4} className='m-0'>
            {isEdit ? t('更新令牌信息') : t('创建新的令牌')}
          </Title>
        </Space>
      }
      bodyStyle={{ padding: '0' }}
      visible={props.visiable}
      width={isMobile ? '100%' : 600}
      footer={
        <div className='flex justify-end bg-white'>
          <Space>
            <Button
              theme='solid'
              className='!rounded-lg'
              onClick={() => formApiRef.current?.submitForm()}
              icon={<IconSave />}
              loading={loading}
            >
              {t('提交')}
            </Button>
            <Button
              theme='light'
              className='!rounded-lg'
              type='primary'
              onClick={handleCancel}
              icon={<IconClose />}
            >
              {t('取消')}
            </Button>
          </Space>
        </div>
      }
      closeIcon={null}
      onCancel={() => handleCancel()}
    >
      <Spin spinning={loading}>
        <Form
          key={isEdit ? 'edit' : 'new'}
          initValues={getInitValues()}
          getFormApi={(api) => (formApiRef.current = api)}
          onSubmit={submit}
        >
          {({ values }) => {
            const marketplacePoolRouteEnabled =
              marketplaceRouteEnabledValue.includes('pool');

            return (
              <div className='p-2'>
                {/* 基本信息 */}
                <Card className='!rounded-2xl shadow-sm border-0'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='blue'
                      className='mr-2 shadow-md'
                    >
                      <IconKey size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('基本信息')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('设置令牌的基本信息')}
                      </div>
                    </div>
                  </div>
                  <Row gutter={12}>
                    <Col span={24}>
                      <Form.Input
                        field='name'
                        label={t('名称')}
                        placeholder={t('请输入名称')}
                        rules={[{ required: true, message: t('请输入名称') }]}
                        showClear
                      />
                    </Col>
                    <Col span={24}>
                      {groups.length > 0 ? (
                        <Form.Select
                          field='group'
                          label={t('令牌分组')}
                          placeholder={t('请选择用户可选分组')}
                          optionList={groups}
                          renderOptionItem={renderGroupOption}
                          filter={selectFilter}
                          showClear
                          style={{ width: '100%' }}
                        />
                      ) : (
                        <Form.Slot label={t('令牌分组')}>
                          <Text type='tertiary'>{t('没有')}</Text>
                        </Form.Slot>
                      )}
                    </Col>
                    <Col span={24}>
                      <Form.Select
                        field='marketplace_fixed_order_ids'
                        label={t('绑定市场买断订单')}
                        placeholder={t('不绑定，可选择多个市场买断订单')}
                        optionList={marketplaceFixedOrderOptions}
                        extraText={t(
                          '可选。绑定后，使用该令牌调用对应市场买断订单时不需要手动填写订单请求头',
                        )}
                        multiple
                        filter={selectFilter}
                        showClear
                        autoClearSearchValue={false}
                        searchPosition='dropdown'
                        style={{ width: '100%' }}
                      />
                      <Text
                        type={
                          marketplacePoolRouteEnabled ? 'success' : 'tertiary'
                        }
                      >
                        {marketplacePoolRouteEnabled
                          ? t('订单池已激活')
                          : t('订单池未激活')}
                      </Text>
                    </Col>
                    <Col span={24}>
                      <Form.Slot
                        label={t('令牌路由优先级')}
                        extraText={t(
                          '已启用路由会按列表顺序尝试。默认顺序：市场买断订单、普通分组订单、订单池',
                        )}
                      >
                        <div className='space-y-2'>
                          {marketplaceRouteOrderValue.map(
                            (route, index, routeOrder) => {
                              const enabledRoutes =
                                marketplaceRouteEnabledValue;
                              const enabled = enabledRoutes.includes(route);

                              return (
                                <div
                                  key={route}
                                  className={`flex items-center gap-3 rounded-lg border p-3 ${
                                    enabled
                                      ? 'bg-white'
                                      : 'bg-gray-50 text-gray-500'
                                  }`}
                                >
                                  <div className='flex h-8 w-8 shrink-0 items-center justify-center rounded-md border text-sm font-medium'>
                                    {index + 1}
                                  </div>
                                  <div className='min-w-0 flex-1'>
                                    <div className='truncate text-sm font-medium'>
                                      {t(MARKETPLACE_ROUTE_ORDER_LABELS[route])}
                                    </div>
                                  </div>
                                  <Switch
                                    checked={enabled}
                                    onChange={(checked) =>
                                      handleMarketplaceRouteEnabledChange(
                                        route,
                                        checked,
                                      )
                                    }
                                    aria-label={t('启用{{route}}路由', {
                                      route: t(
                                        MARKETPLACE_ROUTE_ORDER_LABELS[route],
                                      ),
                                    })}
                                  />
                                  <Space spacing={4}>
                                    <Button
                                      type='tertiary'
                                      theme='borderless'
                                      icon={<IconChevronUp />}
                                      disabled={index === 0}
                                      onClick={() =>
                                        handleMarketplaceRouteOrderMove(
                                          index,
                                          -1,
                                        )
                                      }
                                      aria-label={t('上移路由')}
                                    />
                                    <Button
                                      type='tertiary'
                                      theme='borderless'
                                      icon={<IconChevronDown />}
                                      disabled={index === routeOrder.length - 1}
                                      onClick={() =>
                                        handleMarketplaceRouteOrderMove(
                                          index,
                                          1,
                                        )
                                      }
                                      aria-label={t('下移路由')}
                                    />
                                  </Space>
                                </div>
                              );
                            },
                          )}
                        </div>
                      </Form.Slot>
                    </Col>
                    <Col
                      span={24}
                      style={{
                        display: values.group === 'auto' ? 'block' : 'none',
                      }}
                    >
                      <Form.Switch
                        field='cross_group_retry'
                        label={t('跨分组重试')}
                        size='default'
                        extraText={t(
                          '开启后，当前分组渠道失败时会按顺序尝试下一个分组的渠道',
                        )}
                      />
                    </Col>
                    <Col xs={24} sm={24} md={24} lg={10} xl={10}>
                      <Form.DatePicker
                        field='expired_time'
                        label={t('过期时间')}
                        type='dateTime'
                        placeholder={t('请选择过期时间')}
                        rules={[
                          { required: true, message: t('请选择过期时间') },
                          {
                            validator: (rule, value) => {
                              // 允许 -1 表示永不过期，也允许空值在必填校验时被拦截
                              if (value === -1 || !value)
                                return Promise.resolve();
                              const time = Date.parse(value);
                              if (isNaN(time)) {
                                return Promise.reject(t('过期时间格式错误！'));
                              }
                              if (time <= Date.now()) {
                                return Promise.reject(
                                  t('过期时间不能早于当前时间！'),
                                );
                              }
                              return Promise.resolve();
                            },
                          },
                        ]}
                        showClear
                        style={{ width: '100%' }}
                      />
                    </Col>
                    <Col xs={24} sm={24} md={24} lg={14} xl={14}>
                      <Form.Slot label={t('过期时间快捷设置')}>
                        <Space wrap>
                          <Button
                            theme='light'
                            type='primary'
                            onClick={() => setExpiredTime(0, 0, 0, 0)}
                          >
                            {t('永不过期')}
                          </Button>
                          <Button
                            theme='light'
                            type='tertiary'
                            onClick={() => setExpiredTime(1, 0, 0, 0)}
                          >
                            {t('一个月')}
                          </Button>
                          <Button
                            theme='light'
                            type='tertiary'
                            onClick={() => setExpiredTime(0, 1, 0, 0)}
                          >
                            {t('一天')}
                          </Button>
                          <Button
                            theme='light'
                            type='tertiary'
                            onClick={() => setExpiredTime(0, 0, 1, 0)}
                          >
                            {t('一小时')}
                          </Button>
                        </Space>
                      </Form.Slot>
                    </Col>
                    {!isEdit && (
                      <Col span={24}>
                        <Form.InputNumber
                          field='tokenCount'
                          label={t('新建数量')}
                          min={1}
                          extraText={t('批量创建时会在名称后自动添加随机后缀')}
                          rules={[
                            { required: true, message: t('请输入新建数量') },
                          ]}
                          style={{ width: '100%' }}
                        />
                      </Col>
                    )}
                  </Row>
                </Card>

                {/* 额度设置 */}
                <Card className='!rounded-2xl shadow-sm border-0'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='green'
                      className='mr-2 shadow-md'
                    >
                      <IconCreditCard size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('额度设置')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('设置令牌可用额度和数量')}
                      </div>
                    </div>
                  </div>
                  <Row gutter={12}>
                    <Col span={24}>
                      <Form.InputNumber
                        field='remain_amount'
                        label={t('金额')}
                        prefix={getCurrencyConfig().symbol}
                        placeholder={t('输入金额')}
                        precision={6}
                        disabled={values.unlimited_quota}
                        min={0}
                        step={0.000001}
                        onChange={(val) => {
                          const amount = val === '' || val == null ? 0 : val;
                          formApiRef.current?.setValue('remain_amount', amount);
                          formApiRef.current?.setValue(
                            'remain_quota',
                            displayAmountToQuota(amount),
                          );
                        }}
                        style={{ width: '100%' }}
                        showClear
                      />
                    </Col>
                    <Col span={24}>
                      <div
                        className='text-xs cursor-pointer mt-1'
                        style={{ color: 'var(--semi-color-text-2)' }}
                        onClick={() => setShowQuotaInput((v) => !v)}
                      >
                        {showQuotaInput
                          ? `▾ ${t('收起原生额度输入')}`
                          : `▸ ${t('使用原生额度输入')}`}
                      </div>
                      <div
                        style={{ display: showQuotaInput ? 'block' : 'none' }}
                        className='mt-2'
                      >
                        <Form.InputNumber
                          field='remain_quota'
                          label={t('额度')}
                          placeholder={t('输入额度')}
                          disabled={values.unlimited_quota}
                          min={0}
                          step={500000}
                          rules={
                            values.unlimited_quota
                              ? []
                              : [{ required: true, message: t('请输入额度') }]
                          }
                          onChange={(val) => {
                            const quota = val === '' || val == null ? 0 : val;
                            formApiRef.current?.setValue('remain_quota', quota);
                            formApiRef.current?.setValue(
                              'remain_amount',
                              Number(quotaToDisplayAmount(quota).toFixed(6)),
                            );
                          }}
                          style={{ width: '100%' }}
                          showClear
                        />
                      </div>
                    </Col>
                    <Col span={24}>
                      <Form.Switch
                        field='unlimited_quota'
                        label={t('无限额度')}
                        size='default'
                        extraText={t(
                          '令牌的额度仅用于限制令牌本身的最大额度使用量，实际的使用受到账户的剩余额度限制',
                        )}
                      />
                    </Col>
                  </Row>
                </Card>

                {/* 访问限制 */}
                <Card className='!rounded-2xl shadow-sm border-0'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='purple'
                      className='mr-2 shadow-md'
                    >
                      <IconLink size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('访问限制')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('设置令牌的访问限制')}
                      </div>
                    </div>
                  </div>
                  <Row gutter={12}>
                    <Col span={24}>
                      <Form.Select
                        field='model_limits'
                        label={t('模型限制列表')}
                        placeholder={t(
                          '请选择该令牌支持的模型，留空支持所有模型',
                        )}
                        multiple
                        optionList={models}
                        extraText={t('非必要，不建议启用模型限制')}
                        filter={selectFilter}
                        autoClearSearchValue={false}
                        searchPosition='dropdown'
                        showClear
                        style={{ width: '100%' }}
                      />
                    </Col>
                    <Col span={24}>
                      <Form.TextArea
                        field='allow_ips'
                        label={t('IP白名单（支持CIDR表达式）')}
                        placeholder={t('允许的IP，一行一个，不填写则不限制')}
                        autosize
                        rows={1}
                        extraText={t(
                          '请勿过度信任此功能，IP可能被伪造，请配合nginx和cdn等网关使用',
                        )}
                        showClear
                        style={{ width: '100%' }}
                      />
                    </Col>
                  </Row>
                </Card>
              </div>
            );
          }}
        </Form>
      </Spin>
    </SideSheet>
  );
};

export default EditTokenModal;
