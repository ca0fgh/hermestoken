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

import React from 'react';
import { useTranslation } from 'react-i18next';
import DocumentRenderer from '../../components/common/DocumentRenderer';

// 当管理员未在后台配置隐私政策时，展示以下默认内容（管理员配置的内容优先）。
const DEFAULT_PRIVACY_POLICY = `# 隐私政策

最近更新：${new Date().getFullYear()} 年

本平台尊重并保护您的隐私。本政策说明我们如何收集、使用与保护您的信息。本平台为独立的 AI 模型 API 网关服务，与各 AI 模型厂商无隶属关系。

## 1. 我们收集的信息
- **账户信息**：如用户名、邮箱等您在注册时提供的信息。
- **使用数据**：如调用记录、用量统计与日志，用于计费与服务运营。
- **您配置的凭据**：如您托管的第三方 API Key，将加密存储，仅用于按您的设置转发您自己的请求。

## 2. 信息的使用
我们将上述信息用于：提供和维护服务、计费与额度管理、保障安全与防止滥用、改进服务质量。

## 3. 信息共享与第三方
- 当您发起 API 请求时，请求内容会转发至您所选择的上游模型厂商，并受其各自隐私政策约束。
- 除法律要求或为提供服务所必需外，我们不会向无关第三方出售或分享您的个人信息。

## 4. 数据安全
我们采取合理的技术与管理措施保护您的信息，敏感凭据加密存储。但请注意，任何传输与存储方式都无法保证绝对安全。

## 5. 您的权利
您有权访问、更正或删除您的账户信息，并可随时删除您在本平台配置的 API Key。

## 6. 政策变更
本政策可能不时更新，更新后将在本页面公布。

## 7. 联系我们
如对本隐私政策有任何疑问，请通过本平台公布的联系方式与我们联系。`;

const PrivacyPolicy = () => {
  const { t } = useTranslation();

  return (
    <DocumentRenderer
      apiEndpoint='/api/privacy-policy'
      title={t('隐私政策')}
      cacheKey='privacy_policy'
      emptyMessage={t('加载隐私政策内容失败...')}
      defaultContent={DEFAULT_PRIVACY_POLICY}
    />
  );
};

export default PrivacyPolicy;
