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
import {
  IconDownload,
  IconPlayCircle,
  IconRefresh,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, downloadTextAsFile, showError, showSuccess } from '../../helpers';
import './index.css';

const { Text, Title } = Typography;

const providerOptions = [
  { label: 'OpenAI', value: 'openai' },
  { label: 'Anthropic', value: 'anthropic' },
];

const clientProfileLabelMap = {
  claude_code: 'Claude Code',
};

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
  openai: ['gpt-5.5', 'gpt-5.4'],
  anthropic: ['claude-opus-4-7', 'claude-opus-4-6'],
};

function clientProfileForProvider(provider) {
  const normalizedProvider = String(provider || '').toLowerCase();
  return normalizedProvider === 'anthropic' ? 'claude_code' : '';
}

const groupLabels = {
  core: '基础检测',
  quality: '质量探针',
  security: '安全探针',
  integrity: '完整性探针',
  identity: '身份探针',
  submodel: '子模型探针',
  signature: '签名探针',
  multimodal: '多模态探针',
  canary: '金丝雀基准',
};

const identityStatusLabels = {
  match: '匹配',
  mismatch: '不一致',
  uncertain: '证据不足',
  insufficient_data: '数据不足',
};

const identityStatusColors = {
  match: 'green',
  mismatch: 'red',
  uncertain: 'orange',
  insufficient_data: 'grey',
};

const verdictStatusLabels = {
  clean_match: '三向匹配',
  clean_match_submodel_mismatch: '子模型不一致',
  plain_mismatch: '身份不一致',
  spoof_behavior_induced: '行为诱导伪装',
  spoof_selfclaim_forged: '自报伪装',
  ambiguous: '信号冲突',
  insufficient_data: '数据不足',
};

const verdictStatusColors = {
  clean_match: 'green',
  clean_match_submodel_mismatch: 'orange',
  plain_mismatch: 'red',
  spoof_behavior_induced: 'red',
  spoof_selfclaim_forged: 'red',
  ambiguous: 'orange',
  insufficient_data: 'grey',
};

function formatMetric(value, suffix = '') {
  const numberValue = Number(value);
  if (!Number.isFinite(numberValue) || numberValue <= 0) {
    return '-';
  }
  return `${numberValue.toFixed(numberValue >= 10 ? 0 : 2)}${suffix}`;
}

function reportFinalRating(report) {
  const rating = report?.final_rating || {};
  const score = Number(
    rating.probe_score ??
      report?.probe_score ??
      rating.score ??
      report?.score ??
      0,
  );
  const scoreMax = Number(
    rating.probe_score_max ??
      report?.probe_score_max ??
      rating.score_max ??
      report?.score_max ??
      rating.scoreMax ??
      report?.scoreMax ??
      score,
  );

  return {
    score: Number.isFinite(score) ? score : 0,
    scoreMax: Number.isFinite(scoreMax) ? scoreMax : score,
    conclusion: rating.conclusion || report?.conclusion || '',
    risks: rating.risks || report?.risks || [],
  };
}

function formatProbeScore(rating) {
  if (rating.scoreMax !== rating.score) {
    return `${rating.score}-${rating.scoreMax}`;
  }
  return String(rating.score);
}

function probeRequestTimeout() {
  return 370000;
}

function probeProfileTagColor() {
  return 'red';
}

function buildProbeEvidenceFilename(result) {
  const compactPart = (value, fallback) =>
    String(value || fallback)
      .trim()
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '') || fallback;
  const provider = compactPart(result?.provider, 'provider');
  const model = compactPart(result?.model, 'model');
  const profile = compactPart(result?.probe_profile, 'profile');
  return `token-verification-evidence-${provider}-${model}-${profile}.json`;
}

function probeStatusMeta(record, t) {
  const normalizedStatus =
    record.status || (record.success || record.passed ? 'passed' : 'failed');
  const riskLevel = String(record.risk_level || '').toLowerCase();

  if (record.error_code === 'judge_unconfigured') {
    return {
      color: 'grey',
      label: t('未评分'),
    };
  }
  if (riskLevel === 'high') {
    return {
      color: 'red',
      label: t('高危'),
    };
  }
  if (riskLevel === 'medium') {
    return {
      color: 'orange',
      label: t('中危'),
    };
  }
  if (riskLevel === 'low' && normalizedStatus === 'passed') {
    return {
      color: 'green',
      label: t('低风险'),
    };
  }
  if (normalizedStatus === 'passed') {
    return {
      color: 'green',
      label: t('通过'),
    };
  }
  if (normalizedStatus === 'skipped') {
    return {
      color: 'grey',
      label: t('跳过'),
    };
  }
  if (normalizedStatus === 'neutral') {
    return {
      color: 'blue',
      label: t('信息'),
    };
  }
  if (normalizedStatus === 'warning') {
    return {
      color: 'orange',
      label: t('警告'),
    };
  }
  return {
    color: 'red',
    label: t('失败'),
  };
}

function renderProbeMessage(value, record) {
  const evidence = Array.isArray(record.evidence) ? record.evidence : [];
  if (!value && evidence.length === 0) {
    return '-';
  }
  return (
    <div className='token-verification-probe-message'>
      {value && <Text>{value}</Text>}
      {evidence.slice(0, 3).map((item) => (
        <Text key={item} type='secondary' size='small'>
          {item}
        </Text>
      ))}
    </div>
  );
}

function renderProbeCheckName(name, record, t) {
  const metadataItems = [
    {
      label: t('检测说明'),
      value: record.check_description,
    },
    {
      label: t('检测覆盖'),
      value: record.coverage,
    },
    {
      label: t('检测局限'),
      value: record.limitation,
    },
    {
      label: t('建议动作'),
      value: record.recommended_action,
    },
  ].filter((item) => item.value);
  return (
    <div className='token-verification-check-name'>
      <Text>{name || record.check_key || '-'}</Text>
      {metadataItems.map((item) => (
        <Text
          className='token-verification-check-meta'
          key={item.label}
          type='secondary'
          size='small'
        >
          <span>{item.label}</span>
          {t(item.value)}
        </Text>
      ))}
    </div>
  );
}

function renderProbeScore(value, record) {
  if (record.skipped || record.status === 'skipped') {
    return '-';
  }
  const score = Number(value);
  if (!Number.isFinite(score)) {
    return '-';
  }
  return `${score}/100`;
}

function renderIdentityPredictedFamily(value, record, t) {
  if (
    record.status === 'uncertain' ||
    record.verdict?.status === 'insufficient_data'
  ) {
    return t('证据不足');
  }
  return value || '-';
}

function renderIdentityEvidence(items = [], record = {}, t = (value) => value) {
  const evidence = Array.isArray(items) ? items : [];
  const notes = [...evidence];
  if (record.status === 'uncertain' && notes.length === 0) {
    notes.push(t('当前信号不足，建议重跑完整探针或查看原始响应'));
  }
  if (record.verdict?.status === 'insufficient_data') {
    notes.push(t('交叉判定数据不足，不能作为身份不一致结论'));
  }
  if (notes.length === 0) {
    return '-';
  }
  return notes.slice(0, 3).join('；');
}

function TokenVerification() {
  const { t } = useTranslation();
  const [provider, setProvider] = useState('anthropic');
  const [baseURL, setBaseURL] = useState(providerDefaults.anthropic.baseURL);
  const [apiKey, setApiKey] = useState('');
  const [model, setModel] = useState(providerDefaults.anthropic.model);
  const [probeProfile, setProbeProfile] = useState('full');
  const [probing, setProbing] = useState(false);
  const [probeResult, setProbeResult] = useState(null);

  const selectedReport = probeResult?.report;
  const checklistItems =
    selectedReport?.checklist || probeResult?.results || [];
  const profileOptions = useMemo(
    () => [{ label: t('完整检测'), value: 'full' }],
    [t],
  );
  const profileLabelMap = useMemo(
    () =>
      Object.fromEntries(
        profileOptions.map((option) => [option.value, option.label]),
      ),
    [profileOptions],
  );

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
      probeProfile: probeResult?.probe_profile || probeProfile,
      clientProfile:
        probeResult?.client_profile || clientProfileForProvider(provider),
    }),
    [baseURL, model, probeProfile, probeResult, provider],
  );

  const handleProviderChange = (value) => {
    const nextProvider = String(value || 'openai');
    const previousDefaults = providerDefaults[provider];
    const nextDefaults =
      providerDefaults[nextProvider] || providerDefaults.openai;

    setProvider(nextProvider);
    if (!baseURL.trim() || baseURL === previousDefaults.baseURL) {
      setBaseURL(nextDefaults.baseURL);
    }
    setModel(nextDefaults.model);
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
          probe_profile: probeProfile,
          client_profile: clientProfileForProvider(provider),
        },
        {
          timeout: probeRequestTimeout(probeProfile),
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

  function exportProbeEvidence() {
    if (!probeResult) {
      return;
    }
    downloadTextAsFile(
      `${JSON.stringify(probeResult, null, 2)}\n`,
      buildProbeEvidenceFilename(probeResult),
    );
  }

  const checklistColumns = [
    {
      title: t('分组'),
      dataIndex: 'group',
      width: 120,
      render: (value) => t(groupLabels[value] || value || '基础检测'),
    },
    {
      title: t('检测项'),
      dataIndex: 'check_name',
      render: (name, record) => renderProbeCheckName(name, record, t),
    },
    {
      title: t('结果'),
      dataIndex: 'status',
      width: 96,
      render: (_status, record) => {
        const meta = probeStatusMeta(record, t);
        return <Tag color={meta.color}>{meta.label}</Tag>;
      },
    },
    {
      title: t('分数'),
      dataIndex: 'score',
      width: 80,
      render: renderProbeScore,
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
      render: renderProbeMessage,
    },
  ];

  const identityColumns = [
    {
      title: t('模型'),
      dataIndex: 'model_name',
      render: (value, record) => value || record.claimed_model || '-',
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 96,
      render: (value) => (
        <Tag color={identityStatusColors[value] || 'grey'}>
          {t(identityStatusLabels[value] || value || '不确定')}
        </Tag>
      ),
    },
    {
      title: t('预测家族'),
      dataIndex: 'predicted_family',
      width: 120,
      render: (value, record) =>
        renderIdentityPredictedFamily(value, record, t),
    },
    {
      title: t('置信度'),
      dataIndex: 'confidence',
      width: 96,
      render: (value) =>
        Number.isFinite(Number(value))
          ? `${Math.round(Number(value) * 100)}%`
          : '-',
    },
    {
      title: t('子模型判定'),
      dataIndex: 'submodel_assessments',
      render: (items = []) => {
        if (!Array.isArray(items) || items.length === 0) {
          return '-';
        }
        return (
          <div className='token-verification-inline-tags'>
            {items.map((item) => (
              <Tag
                key={item.method}
                color={item.abstained ? 'grey' : 'light-blue'}
              >
                {String(item.method || '').toUpperCase()}
                {item.display_name ? ` ${item.display_name}` : ''}
              </Tag>
            ))}
          </div>
        );
      },
    },
    {
      title: t('交叉判定'),
      dataIndex: 'verdict',
      width: 136,
      render: (value) => {
        const status = value?.status;
        if (!status) {
          return '-';
        }
        return (
          <Tag color={verdictStatusColors[status] || 'grey'}>
            {t(verdictStatusLabels[status] || status)}
          </Tag>
        );
      },
    },
    {
      title: t('证据'),
      dataIndex: 'evidence',
      render: (value, record) => renderIdentityEvidence(value, record, t),
    },
  ];

  const renderReport = () => {
    if (probing) {
      return (
        <div className='token-verification-loading'>
          <Spin size='large' />
          <Text type='secondary'>
            {t('正在检测中，通常需要几十秒，完整检测可能需要数分钟。')}
          </Text>
        </div>
      );
    }

    if (!selectedReport) {
      return (
        <Empty
          title={t('暂无探针报告')}
          description={t('填写目标后开始运行探针套件')}
        />
      );
    }

    const rating = reportFinalRating(selectedReport);
    const risks = rating.risks || selectedReport.risks || [];
    const identityAssessments = selectedReport.identity_assessments || [];

    return (
      <div className='token-verification-report'>
        <div className='token-verification-score'>
          <div>
            <Text type='secondary'>{t('探针评分')}</Text>
            <div className='token-verification-score__value'>
              {formatProbeScore(rating)}
              <span>/100</span>
            </div>
          </div>
          <Text>{rating.conclusion || selectedReport.conclusion || '-'}</Text>
        </div>

        <div className='token-verification-result-meta'>
          <Tag color='light-blue'>{selectedTarget.provider}</Tag>
          <Tag color='light-blue'>{selectedTarget.model}</Tag>
          <Tag color={probeProfileTagColor(selectedTarget.probeProfile)}>
            {profileLabelMap[selectedTarget.probeProfile] ||
              selectedTarget.probeProfile}
          </Tag>
          {selectedTarget.clientProfile && (
            <Tag color='violet'>
              {clientProfileLabelMap[selectedTarget.clientProfile] ||
                selectedTarget.clientProfile}
            </Tag>
          )}
          <Text type='secondary'>{selectedTarget.baseURL}</Text>
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

        {identityAssessments.length > 0 && (
          <div className='token-verification-section'>
            <Title heading={5}>{t('身份指纹评估')}</Title>
            <Table
              size='small'
              columns={identityColumns}
              dataSource={identityAssessments}
              pagination={false}
              rowKey={(record, index) =>
                `${record.provider}:${record.model_name}:${index}`
              }
            />
          </div>
        )}

        <div className='token-verification-section'>
          <Title heading={5}>{t('探针清单')}</Title>
          <Table
            size='small'
            columns={checklistColumns}
            dataSource={checklistItems}
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
            <Title heading={3}>{t('LLM 探针检测')}</Title>
            <Text type='secondary'>
              {t('基于探针套件发起真实请求，生成身份、完整性与安全检测报告。')}
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
          <Button
            type='tertiary'
            icon={<IconDownload />}
            disabled={!probeResult || probing}
            onClick={exportProbeEvidence}
          >
            {t('导出证据')}
          </Button>
        </div>

        <div className='token-verification-grid'>
          <Card
            className='token-verification-panel'
            title={t('检测配置')}
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

              {provider === 'anthropic' && (
                <label>
                  <Text strong>{t('客户端模式')}</Text>
                  <Input
                    readOnly
                    value={clientProfileLabelMap.claude_code}
                    onChange={() => {}}
                  />
                </label>
              )}

              <label>
                <Text strong>{t('检测模型')}</Text>
                <Select
                  key={`model-select-${provider}`}
                  allowCreate
                  filter
                  optionList={modelOptions}
                  value={model}
                  onChange={(value) => setModel(String(value || ''))}
                  placeholder='gpt-5.5'
                  style={{ width: '100%' }}
                />
              </label>

              <label>
                <Text strong>{t('检测深度')}</Text>
                <Select
                  optionList={profileOptions}
                  value={probeProfile}
                  onChange={(value) => setProbeProfile(String(value || 'full'))}
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

          <Card className='token-verification-panel' title={t('探针报告')}>
            {renderReport()}
          </Card>
        </div>
      </div>
    </main>
  );
}

export default TokenVerification;
