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
  lazy,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { useActualTheme } from '../../context/Theme';
import { useTranslation } from 'react-i18next';
import { showError } from '../../helpers/notifications';

const MarketingNoticeModal = lazy(
  () => import('../../components/layout/MarketingNoticeModal'),
);


async function fetchHomePayload(path) {
  const response = await fetch(path, {
    headers: {
      'Cache-Control': 'no-store',
    },
  });

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }

  return response.json();
}

async function renderHomePageMarkdown(markdown) {
  const { marked } = await import('marked');
  return marked.parse(markdown);
}

const Home = () => {
  const { t, i18n } = useTranslation();
  const actualTheme = useActualTheme();
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(false);
  const [homePageContent, setHomePageContent] = useState(() => {
    try {
      return localStorage.getItem('home_page_content') || '';
    } catch {
      return '';
    }
  });
  const [noticeVisible, setNoticeVisible] = useState(false);
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
    if (!homePageContent.startsWith('https://')) {
      return;
    }
    const iframe = homepageIframeRef.current;
    if (!iframe?.contentWindow) {
      return;
    }
    iframe.contentWindow.postMessage({ themeMode: actualTheme }, '*');
    iframe.contentWindow.postMessage({ lang: i18n.language }, '*');
  }, [actualTheme, homePageContent, i18n.language]);

  const displayHomePageContent = async () => {
    let cachedHomePageContent = '';
    try {
      cachedHomePageContent = localStorage.getItem('home_page_content') || '';
    } catch {
      cachedHomePageContent = '';
    }
    try {
      const { success, message, data } = await fetchHomePayload(
        '/api/home_page_content',
      );
      if (success) {
        let content = data;
        if (!data.startsWith('https://')) {
          content = await renderHomePageMarkdown(data);
        }
        setHomePageContent(content);
        try {
          localStorage.setItem('home_page_content', content);
        } catch {
          // Ignore storage write failures and keep the in-memory homepage state.
        }
      } else {
        showError(message);
        setHomePageContent('');
        try {
          localStorage.removeItem('home_page_content');
        } catch {
          // Ignore storage removal failures and continue with the default homepage.
        }
      }
    } catch (error) {
      showError(error?.message || '加载首页内容失败');
      if (!cachedHomePageContent) {
        setHomePageContent('');
      }
    } finally {
      setHomePageContentLoaded(true);
    }
  };

  useEffect(() => {
    const checkNoticeAndShow = async () => {
      const lastCloseDate = localStorage.getItem('notice_close_date');
      const today = new Date().toDateString();
      if (lastCloseDate !== today) {
        try {
          const { success, data } = await fetchHomePayload('/api/notice');
          if (success && data && data.trim() !== '') {
            setNoticeVisible(true);
          }
        } catch (error) {
          console.error('获取公告失败:', error);
        }
      }
    };

    checkNoticeAndShow();
  }, []);

  useEffect(() => {
    void displayHomePageContent();
  }, []);

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
      {homePageContent !== '' ? (
        <div className='overflow-x-hidden w-full'>
          {homePageContent.startsWith('https://') ? (
            <iframe
              ref={homepageIframeRef}
              src={homePageContent}
              onLoad={syncIframeThemeAndLanguage}
              className='w-full h-screen border-none'
            />
          ) : (
            <div
              className='mt-[60px]'
              dangerouslySetInnerHTML={{ __html: homePageContent }}
            />
          )}
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
