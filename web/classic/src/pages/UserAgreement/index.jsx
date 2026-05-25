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

// 当管理员未在后台配置用户协议时，展示以下默认条款（管理员配置的内容优先）。
const DEFAULT_USER_AGREEMENT = `# 用户协议

最近更新：${new Date().getFullYear()} 年

欢迎使用本服务（以下简称“本平台”）。本平台为第三方独立服务。在使用本平台前，请仔细阅读并同意以下条款。

## 1. 适用地区与合规
- 本平台聚合的部分大语言模型由境外厂商提供；本平台不面向中国大陆境内公众提供生成式人工智能服务。
- 您在使用前应自行确认：所在国家或地区的法律法规允许您访问并使用本平台及相关模型。能否使用、使用是否合规，由您自行判断并承担责任。
- 若您所在地区对相关服务设有准入、备案要求或限制、禁止，请勿注册或使用本平台；由此产生的一切后果由您自行承担。
- 本平台有权基于合规要求，对特定地区、模型或功能进行限制或停止提供。

## 2. 服务说明
本平台提供大语言模型 API 的聚合、分发与管理服务，将多种模型统一为标准化接口。本平台不拥有任何第三方模型。

## 3. 账户与密钥
- 您需对自己账户下的所有活动负责，并妥善保管账户凭据。
- 如您在本平台配置自有的第三方 API Key，您确认拥有合法使用与共享该 Key 的权利，并自行遵守对应厂商的服务条款。
- 部分模型厂商的条款可能限制 API Key 的共享、转售或代理使用。是否在本平台配置、共享或交易您的 Key 由您自行判断；由此引发的与厂商之间的纠纷、封禁或损失，由您自行承担，本平台不承担责任。
- 您配置的 API Key 仅用于按您的设置转发您自己的请求，加密存储，不会公开展示给他人。

## 4. 使用规范
您承诺不利用本平台从事任何违反法律法规的活动，包括但不限于生成违法内容、侵犯他人合法权益、滥用或攻击服务等。

## 5. 免责声明
本平台按“现状”与“现有”提供服务，不对服务的连续性、及时性、安全性，以及第三方模型输出的准确性、合法性或适用性作出任何明示或默示担保。您通过本平台输入、生成或传输的全部内容由您自行负责，本平台不对其合法性及由此产生的后果承担责任。在法律允许的最大范围内，本平台不对任何间接、附带、惩罚性或衍生性损失承担责任。

## 6. 责任限制与赔偿
在法律允许的范围内，本平台就本服务对您承担的累计责任，以您为相关服务实际支付的费用为限。因您违反本协议或法律法规，或因您配置、共享、交易的 API Key 或您的使用行为引发的任何第三方索赔、纠纷或损失，由您自行承担，并赔偿因此给本平台造成的损失（含合理的律师费用）。

## 7. 条款变更
本平台可能不时更新本协议，更新后将在本页面公布。您继续使用本服务即视为接受变更后的条款。

## 8. 联系我们
如对本协议有任何疑问，请通过本平台公布的联系方式与我们联系。`;

const UserAgreement = () => {
  const { t } = useTranslation();

  return (
    <DocumentRenderer
      apiEndpoint='/api/user-agreement'
      title={t('用户协议')}
      cacheKey='user_agreement'
      emptyMessage={t('加载用户协议内容失败...')}
      defaultContent={DEFAULT_USER_AGREEMENT}
    />
  );
};

export default UserAgreement;
