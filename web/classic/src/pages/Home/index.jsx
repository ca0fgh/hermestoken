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

import React, {
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';
import { removePublicHomeShell } from '../../bootstrap/publicStartup';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { useActualTheme } from '../../context/Theme';
import { useTranslation } from 'react-i18next';
import { lazyWithRetry } from '../../helpers/lazyWithRetry';
import { cachePublicBootstrap } from '../../helpers/publicStartupCache';
import { scheduleNonCriticalWork } from '../../helpers/idleTask';
import { readStoredValue } from '../../helpers/storageJson';
import { fetchPublicBootstrap } from './publicBootstrapRefresh';
import { resolveHomeStartupBootstrap } from './startupBootstrap';

const MarketingNoticeModal = lazyWithRetry(
  () => import('../../components/layout/MarketingNoticeModal'),
  'marketing-notice-modal',
);

const startupBootstrap = resolveHomeStartupBootstrap();

function isNoticeDismissedToday() {
  const lastCloseDate = readStoredValue('notice_close_date', '');
  return lastCloseDate === new Date().toDateString();
}

function hasBootstrapNotice(notice) {
  return Boolean(notice?.html || notice?.markdown);
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
    hasBootstrapNotice(startupBootstrap?.notice) && !isNoticeDismissedToday(),
  );
  const [noticeHtml, setNoticeHtml] = useState(
    startupBootstrap?.notice?.html || '',
  );
  const homepageIframeRef = useRef(null);
  const isMobile = useIsMobile();
  const homeNarrativeTags = [
    t('能力流动性'),
    t('供需网络'),
    t('执行秩序'),
    t('可信结算'),
  ];
  const homeNarrativeCards = [
    {
      eyebrow: t('供给抽象'),
      title: t('将异构智能能力抽象为流动性层'),
      body: t(
        '不同来源的智能能力被统一表达为可供给、可组合、可约束、可履约的能力单元。',
      ),
    },
    {
      eyebrow: t('流动网络'),
      title: t('支撑 AGI 的连续能力获取'),
      body: t(
        'AGI 不再绑定静态资源，而是在统一供需网络中按场景获得持续、稳定的能力供给。',
      ),
    },
    {
      eyebrow: t('执行秩序'),
      title: t('让能力流动具备确定性'),
      body: t(
        '发现、调度、执行、结算与审计被纳入同一秩序，支撑能力在多主体之间规模化流动。',
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

  useEffect(() => {
    removePublicHomeShell();
  }, []);

  const applyBootstrap = useCallback((payload) => {
    if (!payload) {
      return;
    }

    if (payload.home?.mode === 'iframe') {
      setHomePageFrameUrl(payload.home?.url || '');
      setHomePageContent('');
    } else {
      setHomePageFrameUrl('');
      setHomePageContent(payload.home?.html || '');
    }

    setNoticeHtml(payload.notice?.html || '');
    setNoticeVisible(
      hasBootstrapNotice(payload.notice) && !isNoticeDismissedToday(),
    );
    setHomePageContentLoaded(true);
  }, []);

  useEffect(() => {
    if (startupBootstrap) {
      applyBootstrap(startupBootstrap);
    }
  }, [applyBootstrap]);

  useEffect(
    () =>
      scheduleNonCriticalWork(async () => {
        try {
          const payload = await fetchPublicBootstrap();

          if (payload?.success && payload?.data) {
            cachePublicBootstrap(payload.data);
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
      }),
    [applyBootstrap],
  );

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
            initialNoticeHtml={noticeHtml}
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
        <div className='w-full border-b border-[#e2e8f0] min-h-[500px] md:min-h-[600px] lg:min-h-[700px] relative overflow-x-hidden'>
          <div className='blur-ball blur-ball-indigo' />
          <div className='blur-ball blur-ball-teal' />
          <div className='flex justify-center h-full px-4 pt-16 pb-20 md:pt-20 md:pb-24 lg:pt-24 lg:pb-28 mt-10'>
            <p className='text-base md:text-lg text-[#64748b]'>
              {t('加载中...')}
            </p>
          </div>
        </div>
      ) : (
        <div className='w-full border-b border-[#e2e8f0] min-h-[500px] md:min-h-[600px] lg:min-h-[700px] relative overflow-x-hidden'>
          <div className='blur-ball blur-ball-indigo' />
          <div className='blur-ball blur-ball-teal' />
          <div className='flex justify-center h-full px-4 pt-16 pb-20 md:pt-20 md:pb-24 lg:pt-24 lg:pb-28 mt-10'>
            <div className='w-full max-w-5xl mx-auto'>
              <section className='text-center'>
                <p className='text-sm md:text-base text-[#64748b] mb-4'>
                  {t('AGI 能力流动性基础设施')}
                </p>
                <h1 className='text-3xl md:text-5xl lg:text-6xl font-bold text-[#020617] leading-tight'>
                  {t('面向 AGI 能力流动性的基础设施')}
                </h1>
                <div className='mt-6 space-y-3 max-w-3xl mx-auto'>
                  <p className='text-base md:text-lg text-[#334155]'>
                    {t(
                      'HermesToken 将异构智能能力抽象为可发现、可组合、可调度、可结算的流动性层。',
                    )}
                  </p>
                  <p className='text-base md:text-lg text-[#334155]'>
                    {t(
                      '让 AGI 在统一供需网络中获得连续、可信、可审计的能力供给。',
                    )}
                  </p>
                </div>
              </section>

              <section className='mt-7 md:mt-8'>
                <div className='flex flex-wrap justify-center gap-2 md:gap-3'>
                  {homeNarrativeTags.map((tag) => (
                    <span
                      key={tag}
                      className='px-3 py-1.5 rounded-full border border-[#cbd5e1cc] text-sm text-[#334155] bg-[#ffffff73]'
                    >
                      {tag}
                    </span>
                  ))}
                </div>
              </section>

              <section className='mt-8 md:mt-10 grid grid-cols-1 md:grid-cols-3 gap-4 md:gap-6'>
                {homeNarrativeCards.map((card) => (
                  <article
                    key={card.eyebrow}
                    className='rounded-2xl border border-[#e2e8f0] bg-[#ffffffb3] p-5 md:p-6'
                  >
                    <p className='text-sm text-[#64748b]'>{card.eyebrow}</p>
                    <h2 className='mt-2 text-lg md:text-xl font-semibold text-[#020617]'>
                      {card.title}
                    </h2>
                    <p className='mt-3 text-sm md:text-base text-[#334155] leading-relaxed'>
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
