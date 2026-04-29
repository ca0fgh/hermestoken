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

import React, { useContext, useEffect, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
import { API } from '../../helpers/api';
import { showError } from '../../helpers/notifications';
import { writeStoredValue } from '../../helpers/storageJson';
import { sanitizeNoticeHtml } from '../../helpers/noticeHtml';
import { getRelativeTime } from '../../helpers/time';
import { StatusContext } from '../../context/Status';
import { shouldFetchMarketingNotice } from './marketingNoticeFetchGate';
import { marked } from 'marked';
import { Bell, Megaphone, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const tabButtonClass = (active) =>
  `inline-flex items-center gap-1 rounded-full px-3 py-1.5 text-sm font-medium transition ${
    active
      ? 'bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900'
      : 'bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700'
  }`;

const MarketingNoticeModal = ({
  visible,
  onClose,
  isMobile,
  defaultTab = 'inApp',
  initialNoticeHtml = '',
  unreadKeys = [],
}) => {
  const { t } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const [noticeContent, setNoticeContent] = useState(() =>
    sanitizeNoticeHtml(initialNoticeHtml),
  );
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState(defaultTab);
  const isDarkMode =
    typeof document !== 'undefined' &&
    document.documentElement.classList.contains('dark');
  const surfaceColor = isDarkMode ? '#020617' : '#ffffff';

  const announcements = statusState?.status?.announcements || [];
  const unreadSet = useMemo(() => new Set(unreadKeys), [unreadKeys]);

  const getKeyForItem = (item) =>
    `${item?.publishDate || ''}-${(item?.content || '').slice(0, 30)}`;

  const processedAnnouncements = useMemo(
    () =>
      (announcements || []).slice(0, 20).map((item) => {
        const pubDate = item?.publishDate ? new Date(item.publishDate) : null;
        const absoluteTime =
          pubDate && !isNaN(pubDate.getTime())
            ? `${pubDate.getFullYear()}-${String(pubDate.getMonth() + 1).padStart(2, '0')}-${String(pubDate.getDate()).padStart(2, '0')} ${String(pubDate.getHours()).padStart(2, '0')}:${String(pubDate.getMinutes()).padStart(2, '0')}`
            : item?.publishDate || '';

        return {
          key: getKeyForItem(item),
          time: absoluteTime,
          content: sanitizeNoticeHtml(marked.parse(item.content || '')),
          extra: item.extra ? sanitizeNoticeHtml(marked.parse(item.extra)) : '',
          relative: getRelativeTime(item.publishDate),
          isUnread: unreadSet.has(getKeyForItem(item)),
        };
      }),
    [announcements, unreadSet],
  );

  useEffect(() => {
    if (!visible) {
      return undefined;
    }

    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';

    const handleEsc = (event) => {
      if (event.key === 'Escape') {
        onClose();
      }
    };

    document.addEventListener('keydown', handleEsc);

    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener('keydown', handleEsc);
    };
  }, [visible, onClose]);

  useEffect(() => {
    if (!visible) {
      return;
    }

    setActiveTab(defaultTab);
  }, [defaultTab, visible]);

  useEffect(() => {
    setNoticeContent(sanitizeNoticeHtml(initialNoticeHtml));
  }, [initialNoticeHtml]);

  useEffect(() => {
    if (!shouldFetchMarketingNotice({ visible, initialNoticeHtml })) {
      setLoading(false);
      return;
    }

    const displayNotice = async () => {
      setLoading(true);
      try {
        const res = await API.get('/api/notice');
        const { success, message, data } = res.data;
        if (!success) {
          showError(message);
          return;
        }

        if (!data) {
          setNoticeContent('');
          return;
        }

        setNoticeContent(sanitizeNoticeHtml(marked.parse(data)));
      } catch (error) {
        showError(error.message);
      } finally {
        setLoading(false);
      }
    };

    void displayNotice();
  }, [initialNoticeHtml, visible]);

  if (!visible) {
    return null;
  }

  const handleCloseTodayNotice = () => {
    const today = new Date().toDateString();
    writeStoredValue('notice_close_date', today);
    onClose();
  };

  const renderEmptyState = (text) => (
    <div className='flex min-h-[220px] items-center justify-center rounded-2xl border border-dashed border-slate-200 bg-white px-6 text-center text-sm text-slate-500 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-400'>
      {text}
    </div>
  );

  const renderInAppNotice = () => {
    if (loading) {
      return renderEmptyState(t('加载中...'));
    }

    if (!noticeContent) {
      return renderEmptyState(t('暂无公告'));
    }

    return (
      <div
        className='notice-content-scroll max-h-[55vh] overflow-y-auto rounded-2xl border border-slate-200 bg-white p-5 text-slate-700 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200'
        style={{ backgroundColor: surfaceColor }}
        dangerouslySetInnerHTML={{ __html: noticeContent }}
      />
    );
  };

  const renderAnnouncements = () => {
    if (processedAnnouncements.length === 0) {
      return renderEmptyState(t('暂无系统公告'));
    }

    return (
      <div className='card-content-scroll max-h-[55vh] overflow-y-auto pr-1'>
        <div className='space-y-4'>
          {processedAnnouncements.map((item) => (
            <article
              key={item.key}
              className='rounded-2xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-950'
              style={{ backgroundColor: surfaceColor }}
            >
              <div className='flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400'>
                <span>{`${item.relative ? `${item.relative} ` : ''}${item.time}`}</span>
                {item.isUnread ? (
                  <span className='rounded-full bg-rose-500/10 px-2 py-0.5 font-semibold text-rose-600 dark:text-rose-300'>
                    {t('未读')}
                  </span>
                ) : null}
              </div>
              <div
                className={`mt-3 text-sm leading-7 text-slate-700 dark:text-slate-200 ${item.isUnread ? 'shine-text' : ''}`}
                dangerouslySetInnerHTML={{ __html: item.content }}
              />
              {item.extra ? (
                <div
                  className='mt-3 text-xs text-slate-500 dark:text-slate-400'
                  dangerouslySetInnerHTML={{ __html: item.extra }}
                />
              ) : null}
            </article>
          ))}
        </div>
      </div>
    );
  };

  const modalContent = (
    <div
      className='fixed inset-0 z-[130] flex items-center justify-center bg-slate-950/55 px-4 py-6'
      style={{ backgroundColor: 'rgba(2, 6, 23, 0.55)' }}
      onClick={onClose}
    >
      <div
        className={`w-full overflow-hidden rounded-[28px] border border-slate-200 bg-white shadow-2xl dark:border-slate-700 dark:bg-slate-950 ${
          isMobile ? 'max-w-full' : 'max-w-3xl'
        }`}
        style={{ backgroundColor: surfaceColor }}
        onClick={(event) => event.stopPropagation()}
      >
        <div className='flex items-start justify-between gap-4 border-b border-slate-200 px-5 py-4 dark:border-slate-800'>
          <div className='min-w-0'>
            <h2 className='text-lg font-semibold text-slate-900 dark:text-slate-100'>
              {t('系统公告')}
            </h2>
            <div className='mt-3 flex flex-wrap gap-2'>
              <button
                type='button'
                className={tabButtonClass(activeTab === 'inApp')}
                onClick={() => setActiveTab('inApp')}
              >
                <Bell size={14} />
                {t('通知')}
              </button>
              <button
                type='button'
                className={tabButtonClass(activeTab === 'system')}
                onClick={() => setActiveTab('system')}
              >
                <Megaphone size={14} />
                {t('系统公告')}
              </button>
            </div>
          </div>
          <button
            type='button'
            aria-label={t('关闭公告')}
            onClick={onClose}
            className='inline-flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full border border-slate-200 text-slate-500 transition hover:border-slate-300 hover:text-slate-700 dark:border-slate-700 dark:text-slate-400 dark:hover:border-slate-600 dark:hover:text-slate-200'
          >
            <X size={18} />
          </button>
        </div>

        <div className='px-5 py-5'>
          {activeTab === 'inApp' ? renderInAppNotice() : renderAnnouncements()}
        </div>

        <div className='flex flex-wrap justify-end gap-3 border-t border-slate-200 px-5 py-4 dark:border-slate-800'>
          <button
            type='button'
            onClick={handleCloseTodayNotice}
            className='rounded-full border border-slate-200 px-4 py-2 text-sm font-medium text-slate-600 transition hover:border-slate-300 hover:text-slate-900 dark:border-slate-700 dark:text-slate-300 dark:hover:border-slate-600 dark:hover:text-slate-100'
          >
            {t('今日关闭')}
          </button>
          <button
            type='button'
            onClick={onClose}
            className='rounded-full bg-slate-900 px-4 py-2 text-sm font-medium text-white transition hover:bg-slate-800 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white'
          >
            {t('关闭公告')}
          </button>
        </div>
      </div>
    </div>
  );

  return createPortal(modalContent, document.body);
};

export default MarketingNoticeModal;
