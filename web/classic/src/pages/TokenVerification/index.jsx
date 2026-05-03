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

import React, { useMemo, useState } from 'react';
import {
  Button,
  Card,
  Empty,
  Input,
  Select,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconPlayCircle, IconRefresh } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import './index.css';

const { Text, Title } = Typography;

const providerOptions = [
  { label: 'OpenAI', value: 'openai' },
  { label: 'Anthropic', value: 'anthropic' },
];

const providerDefaults = {
  openai: {
    baseURL: 'https://api.openai.com',
    model: 'gpt-5.5',
  },
  anthropic: {
    baseURL: 'https://api.anthropic.com',
    model: 'claude-opus-4-7',
  },
};

const providerModelPresets = {
  openai: ['gpt-5.5', 'gpt-5.4', 'gpt-4o-mini'],
  anthropic: ['claude-opus-4-7', 'claude-opus-4-6', 'claude-3-5-haiku-latest'],
};

const dimensionLabels = {
  availability: '基础可用性',
  model_access: '模型访问',
  model_identity: '身份一致性',
  stability: '稳定性',
  performance: '性能',
  stream: '流式',
  json: 'JSON',
};

function formatMetric(value, suffix = '') {
  const numberValue = Number(value);
  if (!Number.isFinite(numberValue) || numberValue <= 0) {
    return '-';
  }
  return `${numberValue.toFixed(numberValue >= 10 ? 0 : 2)}${suffix}`;
}

function reportFinalRating(report) {
  return (
    report?.final_rating || {
      score: report?.score || 0,
      grade: report?.grade || '',
      conclusion: report?.conclusion || '',
      risks: report?.risks || [],
      dimensions: report?.dimensions || {},
    }
  );
}

function TokenVerification() {
  const { t } = useTranslation();
  const [provider, setProvider] = useState('openai');
  const [baseURL, setBaseURL] = useState(providerDefaults.openai.baseURL);
  const [apiKey, setApiKey] = useState('');
  const [model, setModel] = useState(providerDefaults.openai.model);
  const [probing, setProbing] = useState(false);
  const [probeResult, setProbeResult] = useState(null);

  const selectedReport = probeResult?.report;
  const modelOptions = useMemo(() => {
    const presets = providerModelPresets[provider] || [];
    const options = presets.map((value) => ({ label: value, value }));
    const trimmedModel = model.trim();
    if (trimmedModel && !presets.includes(trimmedModel)) {
      options.unshift({ label: trimmedModel, value: trimmedModel });
    }
    return options;
  }, [model, provider]);
  const selectedTarget = useMemo(
    () => ({
      baseURL: probeResult?.base_url || baseURL,
      provider: probeResult?.provider || provider,
      model: probeResult?.model || model,
    }),
    [baseURL, model, probeResult, provider],
  );

  const handleProviderChange = (value) => {
    const nextProvider = value || 'openai';
    const previousDefaults = providerDefaults[provider];
    const nextDefaults =
      providerDefaults[nextProvider] || providerDefaults.openai;

    setProvider(nextProvider);
    if (!baseURL.trim() || baseURL === previousDefaults.baseURL) {
      setBaseURL(nextDefaults.baseURL);
    }
    if (!model.trim() || model === previousDefaults.model) {
      setModel(nextDefaults.model);
    }
  };

  const createProbe = async () => {
    const trimmedBaseURL = baseURL.trim();
    const trimmedAPIKey = apiKey.trim();
    const trimmedModel = model.trim();

    if (!trimmedBaseURL) {
      showError(t('请输入检测 URL'));
      return;
    }
    if (!trimmedAPIKey) {
      showError(t('请输入 API Key'));
      return;
    }
    if (!trimmedModel) {
      showError(t('请输入检测模型'));
      return;
    }

    setProbing(true);
    setProbeResult(null);
    try {
      const response = await API.post(
        '/api/token_verification/probe',
        {
          base_url: trimmedBaseURL,
          api_key: trimmedAPIKey,
          provider,
          model: trimmedModel,
        },
        {
          timeout: 310000,
        },
      );
      const { success, message, data } = response.data || {};
      if (!success) {
        showError(message || t('检测失败'));
        return;
      }

      setProbeResult(data);
      showSuccess(t('检测完成'));
    } catch (error) {
      showError(error?.message || t('检测失败'));
    } finally {
      setProbing(false);
    }
  };

  const checklistColumns = [
    {
      title: t('检测项'),
      dataIndex: 'check_name',
      render: (name, record) => name || record.check_key,
    },
    {
      title: t('协议'),
      dataIndex: 'provider',
      width: 96,
      render: (value) => value || '-',
    },
    {
      title: t('模型'),
      dataIndex: 'model_name',
      render: (value) => value || '-',
    },
    {
      title: t('结果'),
      dataIndex: 'status',
      width: 96,
      render: (status, record) => {
        const normalizedStatus =
          status || (record.success || record.passed ? 'passed' : 'failed');
        const color =
          normalizedStatus === 'passed'
            ? 'green'
            : normalizedStatus === 'skipped'
              ? 'grey'
              : 'red';
        const label =
          normalizedStatus === 'passed'
            ? t('通过')
            : normalizedStatus === 'skipped'
              ? t('跳过')
              : t('失败');
        return <Tag color={color}>{label}</Tag>;
      },
    },
    {
      title: t('延迟'),
      dataIndex: 'latency_ms',
      width: 90,
      render: (value) => formatMetric(value, 'ms'),
    },
    {
      title: t('说明'),
      dataIndex: 'message',
      render: (value) => value || '-',
    },
  ];

  const modelColumns = [
    {
      title: t('协议'),
      dataIndex: 'provider',
      width: 96,
    },
    {
      title: t('模型'),
      dataIndex: 'model_name',
    },
    {
      title: t('可用'),
      dataIndex: 'available',
      width: 90,
      render: (available) => (
        <Tag color={available ? 'green' : 'red'}>
          {available ? t('可用') : t('不可用')}
        </Tag>
      ),
    },
    {
      title: t('延迟'),
      dataIndex: 'latency_ms',
      width: 90,
      render: (value) => formatMetric(value, 'ms'),
    },
    {
      title: t('首字节'),
      dataIndex: 'stream_ttft_ms',
      width: 90,
      render: (value) => formatMetric(value, 'ms'),
    },
    {
      title: t('速度'),
      dataIndex: 'stream_tokens_ps',
      width: 110,
      render: (value) => formatMetric(value, ' t/s'),
    },
  ];

  const renderReport = () => {
    if (probing) {
      return (
        <div className='token-verification-loading'>
          <Spin size='large' />
          <Text type='secondary'>
            {t('正在检测中，通常需要 1 到 3 分钟。')}
          </Text>
        </div>
      );
    }

    if (!selectedReport) {
      return (
        <Empty
          title={t('暂无检测详情')}
          description={t('输入 URL、API Key 和模型后开始检测')}
        />
      );
    }

    const rating = reportFinalRating(selectedReport);
    const metrics = selectedReport.metrics || {};
    const risks = rating.risks || selectedReport.risks || [];
    const dimensions = rating.dimensions || selectedReport.dimensions || {};

    return (
      <div className='token-verification-report'>
        <div className='token-verification-score'>
          <div>
            <Text type='secondary'>{t('综合评分')}</Text>
            <div className='token-verification-score__value'>
              {rating.score || 0}
              <span>{rating.grade || '-'}</span>
            </div>
          </div>
          <Text>{rating.conclusion || selectedReport.conclusion || '-'}</Text>
        </div>

        <div className='token-verification-result-meta'>
          <Tag color='light-blue'>{selectedTarget.provider}</Tag>
          <Tag color='light-blue'>{selectedTarget.model}</Tag>
          <Text type='secondary'>{selectedTarget.baseURL}</Text>
        </div>

        <div className='token-verification-metrics'>
          <div>
            <Text type='secondary'>{t('平均延迟')}</Text>
            <strong>{formatMetric(metrics.avg_latency_ms, 'ms')}</strong>
          </div>
          <div>
            <Text type='secondary'>{t('平均首字节')}</Text>
            <strong>{formatMetric(metrics.avg_ttft_ms, 'ms')}</strong>
          </div>
          <div>
            <Text type='secondary'>{t('平均输出速度')}</Text>
            <strong>
              {formatMetric(metrics.avg_tokens_per_second, ' t/s')}
            </strong>
          </div>
        </div>

        <div className='token-verification-dimensions'>
          {Object.entries(dimensions).map(([key, value]) => (
            <Tag key={key} color='blue'>
              {t(dimensionLabels[key] || key)} {value}
            </Tag>
          ))}
        </div>

        {risks.length > 0 && (
          <div className='token-verification-alert token-verification-alert--warn'>
            <Text strong>{t('风险提示')}</Text>
            {risks.map((risk) => (
              <Text key={risk} type='secondary'>
                {risk}
              </Text>
            ))}
          </div>
        )}

        <div className='token-verification-section'>
          <Title heading={5}>{t('模型可用性')}</Title>
          <Table
            size='small'
            columns={modelColumns}
            dataSource={selectedReport.models || []}
            pagination={false}
            rowKey={(record) => `${record.provider}:${record.model_name}`}
          />
        </div>

        <div className='token-verification-section'>
          <Title heading={5}>{t('检测清单')}</Title>
          <Table
            size='small'
            columns={checklistColumns}
            dataSource={selectedReport.checklist || probeResult?.results || []}
            pagination={false}
            rowKey={(record, index) =>
              `${record.provider}:${record.check_key}:${record.model_name}:${index}`
            }
          />
        </div>
      </div>
    );
  };

  return (
    <main className='token-verification-page'>
      <div className='token-verification-page__inner'>
        <div className='token-verification-header'>
          <div>
            <Title heading={3}>{t('Token 质量黑盒检测')}</Title>
            <Text type='secondary'>
              {t(
                '输入 API Base URL、API Key 和模型，发起真实黑盒请求并生成质量报告。',
              )}
            </Text>
          </div>
          <Button
            type='tertiary'
            icon={<IconRefresh />}
            disabled={!probeResult || probing}
            onClick={() => setProbeResult(null)}
          >
            {t('清空结果')}
          </Button>
        </div>

        <div className='token-verification-grid'>
          <Card
            className='token-verification-panel'
            title={t('创建检测任务')}
            headerExtraContent={
              <Button
                type='primary'
                icon={<IconPlayCircle />}
                loading={probing}
                onClick={createProbe}
              >
                {t('开始检测')}
              </Button>
            }
          >
            <div className='token-verification-form'>
              <label>
                <Text strong>{t('检测 URL')}</Text>
                <Input
                  value={baseURL}
                  onChange={setBaseURL}
                  placeholder='https://api.example.com/v1'
                />
              </label>

              <label>
                <Text strong>{t('API Key')}</Text>
                <Input
                  mode='password'
                  value={apiKey}
                  onChange={setApiKey}
                  placeholder='sk-...'
                />
              </label>

              <label>
                <Text strong>{t('检测协议')}</Text>
                <Select
                  optionList={providerOptions}
                  value={provider}
                  onChange={handleProviderChange}
                  style={{ width: '100%' }}
                />
              </label>

              <label>
                <Text strong>{t('检测模型')}</Text>
                <Select
                  allowCreate
                  filter
                  optionList={modelOptions}
                  value={model}
                  onChange={(value) => setModel(String(value || ''))}
                  placeholder='gpt-5.5'
                  style={{ width: '100%' }}
                />
              </label>

              <Text
                className='token-verification-secret-note'
                type='secondary'
                size='small'
              >
                {t('API Key 仅用于本次检测请求，不会保存到检测历史或报告。')}
              </Text>
            </div>
          </Card>

          <Card className='token-verification-panel' title={t('检测详情')}>
            {renderReport()}
          </Card>
        </div>
      </div>
    </main>
  );
}

export default TokenVerification;
