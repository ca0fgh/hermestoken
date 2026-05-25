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

import React, { useEffect, useState } from 'react';
import { API } from '../../helpers';
import { marked } from 'marked';

// 管理员未在后台配置「关于」内容时展示的默认介绍（管理员配置的内容优先）。
const DefaultAbout = () => {
  const year = new Date().getFullYear();
  return (
    <div className='mx-auto max-w-2xl px-6 py-16 text-center'>
      <p>关于 HermesToken</p>
      <p>独立的 AI 模型 API 网关服务</p>

      <p>&nbsp;</p>
      <p>
        HermesToken
        帮助个人与企业在一个平台上接入、管理与分发多种大语言模型，把分散的密钥、额度、计费与用量监控集中起来，让多模型的使用更简单、更可控。
      </p>
      <p>
        本服务为第三方独立平台。
      </p>

      <p>&nbsp;</p>
      <p>主要能力</p>
      <p>统一接口：将多种大语言模型转换为标准化 API 格式，一次接入、随处调用。</p>
      <p>精细管控：集中托管密钥，按令牌设置额度、并发与速率限制。</p>
      <p>清晰计费：完整的调用日志与用量统计，成本与流量一目了然。</p>

      <p>&nbsp;</p>
      <p>HermesToken © {year} ｜ 基于 One API v0.5.4 © 2023 JustSong</p>
      <p>本项目根据 MIT 许可证授权，需在遵守 AGPL v3.0 协议的前提下使用。</p>
    </div>
  );
};

const About = () => {
  const [about, setAbout] = useState('');
  const [useDefault, setUseDefault] = useState(false);

  const displayAbout = async () => {
    const cached = localStorage.getItem('about') || '';
    if (cached) {
      setAbout(cached);
    }
    try {
      const res = await API.get('/api/about');
      const { success, data } = res.data;
      if (success && data && data.trim() !== '') {
        const aboutContent = data.startsWith('https://')
          ? data
          : marked.parse(data);
        setAbout(aboutContent);
        localStorage.setItem('about', aboutContent);
      } else if (!cached) {
        setUseDefault(true);
      }
    } catch (error) {
      if (!cached) {
        setUseDefault(true);
      }
    }
  };

  useEffect(() => {
    displayAbout().then();
  }, []);

  // 管理员配置了外部链接
  if (about.startsWith('https://')) {
    return (
      <div className='mt-[60px] px-2'>
        <iframe
          src={about}
          title='About'
          style={{ width: '100%', height: '100vh', border: 'none' }}
        />
      </div>
    );
  }

  // 管理员配置了 HTML / Markdown 内容
  if (about) {
    return (
      <div className='mt-[60px] px-2 pb-16'>
        <div
          className='mx-auto max-w-3xl px-4 text-center'
          dangerouslySetInnerHTML={{ __html: about }}
        ></div>
      </div>
    );
  }

  // 无管理员内容：展示默认的纯文字居中介绍
  if (useDefault) {
    return (
      <div className='mt-[60px]'>
        <DefaultAbout />
      </div>
    );
  }

  return null;
};

export default About;
