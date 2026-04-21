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

import React, {
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { useActualTheme } from '../../context/Theme';
import { useTranslation } from 'react-i18next';
import { lazyWithRetry } from '../../helpers/lazyWithRetry';
import { readInjectedBootstrap } from '../../helpers/bootstrapData';
import {
  cachePublicBootstrap,
  readCachedPublicBootstrap,
} from '../../helpers/publicStartupCache';
import { scheduleNonCriticalWork } from '../../helpers/idleTask';
import { readStoredValue } from '../../helpers/storageJson';

const MarketingNoticeModal = lazyWithRetry(
  () => import('../../components/layout/MarketingNoticeModal'),
  'marketing-notice-modal',
);

const startupBootstrap =
  readInjectedBootstrap() || readCachedPublicBootstrap() || null;

async function fetchPublicBootstrap() {
  const response = await fetch('/api/public/bootstrap', {
    headers: {
      'Cache-Control': 'no-store',
    },
  });

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }

  return response.json();
}

function isNoticeDismissedToday() {
  const lastCloseDate = readStoredValue('notice_close_date', '');
  return lastCloseDate === new Date().toDateString();
}

const Home = () => {
  const { t, i18n } = useTranslation();
  const actualTheme = useActualTheme();
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(
    Boolean(startupBootstrap?.home),
  );
  const [homePageContent, setHomePageContent] = useState(
    startupBootstrap?.home?.mode === 'iframe'
      ? ''
      : startupBootstrap?.home?.html || '',
  );
  const [homePageFrameUrl, setHomePageFrameUrl] = useState(
    startupBootstrap?.home?.mode === 'iframe'
      ? startupBootstrap?.home?.url || ''
      : '',
  );
  const [noticeVisible, setNoticeVisible] = useState(
    Boolean(startupBootstrap?.notice?.markdown) && !isNoticeDismissedToday(),
  );
  const homepageIframeRef = useRef(null);
  const isMobile = useIsMobile();
  const homeNarrativeTags = [
    t('交易层'),
    t('执行层'),
    t('结算层'),
    t('Human 与 Agent'),
  ];
  const homeNarrativeCards = [
    {
      eyebrow: t('标准化'),
      title: t('LLM Token 使用权正在被标准化'),
      body: t(
        '使用能力开始脱离非结构化密钥流转，转向可定价、可约束、可履约的访问权对象。',
      ),
    },
    {
      eyebrow: t('资源化'),
      title: t('AI 能力开始进入资源化阶段'),
      body: t(
        '组织级使用、多模型路由与 Agent 持续调用同时出现，使用权开始需要统一执行与结算边界。',
      ),
    },
    {
      eyebrow: t('协作结构'),
      title: t('买方、卖方、Agent 在同一结构里协作'),
      body: t(
        '平台处理的不是裸 Key，而是带规则的访问能力，让发现、执行和结算进入同一结构。',
      ),
    },
  ];

  const syncIframeThemeAndLanguage = useCallback(() => {
    if (!homePageFrameUrl.startsWith('https://')) {
      return;
    }
    const iframe = homepageIframeRef.current;
    if (!iframe?.contentWindow) {
      return;
    }
    iframe.contentWindow.postMessage({ themeMode: actualTheme }, '*');
    iframe.contentWindow.postMessage({ lang: i18n.language }, '*');
  }, [actualTheme, homePageFrameUrl, i18n.language]);

  const applyBootstrap = useCallback((payload) => {
    if (!payload) {
      return;
    }

    cachePublicBootstrap(payload);

    if (payload.home?.mode === 'iframe') {
      setHomePageFrameUrl(payload.home?.url || '');
      setHomePageContent('');
    } else {
      setHomePageFrameUrl('');
      setHomePageContent(payload.home?.html || '');
    }

    setNoticeVisible(
      Boolean(payload.notice?.markdown) && !isNoticeDismissedToday(),
    );
    setHomePageContentLoaded(true);
  }, []);

  useEffect(() => {
    if (startupBootstrap) {
      applyBootstrap(startupBootstrap);
    }
  }, [applyBootstrap]);

  useEffect(() => (
    scheduleNonCriticalWork(async () => {
      try {
        const payload = await fetchPublicBootstrap();

        if (payload?.success && payload?.data) {
          applyBootstrap(payload.data);
          return;
        }
      } catch (error) {
        console.error('failed to refresh public bootstrap', error);
      }

      if (!startupBootstrap) {
        setHomePageContent('');
        setHomePageFrameUrl('');
        setNoticeVisible(false);
        setHomePageContentLoaded(true);
      }
    })
  ), [applyBootstrap]);

  useEffect(() => {
    syncIframeThemeAndLanguage();
  }, [syncIframeThemeAndLanguage]);

  return (
    <div className='w-full overflow-x-hidden'>
      {noticeVisible ? (
        <Suspense fallback={null}>
          <MarketingNoticeModal
            visible={noticeVisible}
            onClose={() => setNoticeVisible(false)}
            isMobile={isMobile}
          />
        </Suspense>
      ) : null}
      {homePageFrameUrl !== '' ? (
        <div className='overflow-x-hidden w-full'>
          <iframe
            ref={homepageIframeRef}
            src={homePageFrameUrl}
            onLoad={syncIframeThemeAndLanguage}
            className='w-full h-screen border-none'
          />
        </div>
      ) : homePageContent !== '' ? (
        <div className='overflow-x-hidden w-full'>
          <div
            className='mt-[60px]'
            dangerouslySetInnerHTML={{ __html: homePageContent }}
          />
        </div>
      ) : !homePageContentLoaded ? (
        <div className='w-full border-b border-semi-color-border min-h-[500px] md:min-h-[600px] lg:min-h-[700px] relative overflow-x-hidden'>
          <div className='blur-ball blur-ball-indigo' />
          <div className='blur-ball blur-ball-teal' />
          <div className='flex items-center justify-center h-full px-4 py-20 md:py-24 lg:py-32 mt-10'>
            <p className='text-base md:text-lg text-semi-color-text-2'>
              {t('加载中...')}
            </p>
          </div>
        </div>
      ) : (
        <div className='w-full border-b border-semi-color-border min-h-[500px] md:min-h-[600px] lg:min-h-[700px] relative overflow-x-hidden'>
          <div className='blur-ball blur-ball-indigo' />
          <div className='blur-ball blur-ball-teal' />
          <div className='flex items-center justify-center h-full px-4 py-20 md:py-24 lg:py-32 mt-10'>
            <div className='w-full max-w-5xl mx-auto'>
              <section className='text-center'>
                <p className='text-sm md:text-base text-semi-color-text-2 mb-4'>
                  {t('LLM Token 使用权共享基础设施')}
                </p>
                <h1 className='text-3xl md:text-5xl lg:text-6xl font-bold text-semi-color-text-0 leading-tight'>
                  {t('面向 Agent 和 Human 自由交易 LLM Token 使用权的基础设施')}
                </h1>
                <div className='mt-6 space-y-3 max-w-3xl mx-auto'>
                  <p className='text-base md:text-lg text-semi-color-text-1'>
                    {t(
                      'HermesToken 将 LLM Token 使用权定义为可交易、可执行、可结算、可审计的标准化资源。',
                    )}
                  </p>
                  <p className='text-base md:text-lg text-semi-color-text-1'>
                    {t(
                      'Agent 与 Human 在同一市场中发现能力、进入统一执行边界，并围绕真实使用完成结算。',
                    )}
                  </p>
                </div>
              </section>

              <section className='mt-8 md:mt-10'>
                <div className='flex flex-wrap justify-center gap-2 md:gap-3'>
                  {homeNarrativeTags.map((tag) => (
                    <span
                      key={tag}
                      className='px-3 py-1.5 rounded-full border border-semi-color-border text-sm text-semi-color-text-1 bg-semi-color-fill-0'
                    >
                      {tag}
                    </span>
                  ))}
                </div>
              </section>

              <section className='mt-10 md:mt-12 grid grid-cols-1 md:grid-cols-3 gap-4 md:gap-6'>
                {homeNarrativeCards.map((card) => (
                  <article
                    key={card.eyebrow}
                    className='rounded-2xl border border-semi-color-border bg-semi-color-bg-1 p-5 md:p-6'
                  >
                    <p className='text-sm text-semi-color-text-2'>
                      {card.eyebrow}
                    </p>
                    <h2 className='mt-2 text-lg md:text-xl font-semibold text-semi-color-text-0'>
                      {card.title}
                    </h2>
                    <p className='mt-3 text-sm md:text-base text-semi-color-text-1 leading-relaxed'>
                      {card.body}
                    </p>
                  </article>
                ))}
              </section>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default Home;
