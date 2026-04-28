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
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { Link } from 'react-router-dom';
import {
  Bell,
  ChevronDown,
  CreditCard,
  KeyRound,
  Languages,
  LogOut,
  Monitor,
  Moon,
  Settings,
  Sun,
} from 'lucide-react';
import HeaderLogo from './headerbar/HeaderLogo';
import { lazyWithRetry } from '../../helpers/lazyWithRetry';
import { useHeaderBar } from '../../hooks/common/useHeaderBar';
import { useNavigation } from '../../hooks/common/useNavigation';
import { useNotifications } from '../../hooks/common/useNotifications';

const MarketingNoticeModal = lazyWithRetry(
  () => import('./MarketingNoticeModal'),
  'marketing-notice-modal',
);

const themeIconMap = {
  auto: Monitor,
  dark: Moon,
  light: Sun,
};

function hashStringToHue(value) {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash += value.charCodeAt(index) * (index + 1);
  }
  return hash % 360;
}

const MenuButton = React.forwardRef(function MenuButton(
  { icon: Icon, label, onClick, badgeCount = 0, children, isOpen = false },
  ref,
) {
  return (
    <div ref={ref} className='relative'>
      <button
        type='button'
        aria-label={label}
        aria-expanded={isOpen}
        onClick={onClick}
        className='relative inline-flex h-10 w-10 items-center justify-center rounded-full border border-slate-200 bg-white/88 text-slate-700 shadow-sm transition hover:border-slate-300 hover:bg-white dark:border-slate-700 dark:bg-slate-900/88 dark:text-slate-200'
      >
        <Icon size={18} />
        {badgeCount > 0 ? (
          <span className='absolute -right-0.5 -top-0.5 min-w-[18px] rounded-full bg-rose-500 px-1 text-[11px] font-semibold leading-[18px] text-white'>
            {badgeCount > 99 ? '99+' : badgeCount}
          </span>
        ) : null}
      </button>
      {children}
    </div>
  );
});

function DropdownPanel({ open, widthClass = 'w-52', children }) {
  if (!open) {
    return null;
  }

  const isDarkMode =
    typeof document !== 'undefined' &&
    document.documentElement.classList.contains('dark');

  return (
    <div
      className={`absolute right-0 top-full z-[120] mt-2 ${widthClass} overflow-hidden rounded-2xl border border-slate-200 bg-white p-2 shadow-xl dark:border-slate-700 dark:bg-slate-950`}
      style={{
        backgroundColor: isDarkMode ? '#020617' : '#ffffff',
        backdropFilter: 'none',
      }}
    >
      {children}
    </div>
  );
}

function MenuItem({
  onClick,
  children,
  isSelected = false,
  destructive = false,
}) {
  const toneClass = isSelected
    ? 'bg-sky-50 text-sky-700 dark:bg-sky-500/15 dark:text-sky-200'
    : destructive
      ? 'text-rose-600 hover:bg-rose-50 dark:text-rose-300 dark:hover:bg-rose-500/10'
      : 'text-slate-700 hover:bg-slate-100 dark:text-slate-200 dark:hover:bg-slate-800';

  return (
    <button
      type='button'
      onClick={onClick}
      className={`flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm transition ${toneClass}`}
    >
      {children}
    </button>
  );
}

const MarketingHeaderBar = () => {
  const {
    userState,
    statusState,
    isMobile,
    currentLang,
    isLoading,
    logo,
    isSelfUseMode,
    docsLink,
    isDemoSiteMode,
    theme,
    headerNavModules,
    pricingRequireAuth,
    logout,
    handleLanguageChange,
    handleThemeToggle,
    navigate,
    t,
  } = useHeaderBar({ onMobileMenuToggle: () => {}, drawerOpen: false });

  const {
    noticeVisible,
    unreadCount,
    handleNoticeOpen,
    handleNoticeClose,
    getUnreadKeys,
  } = useNotifications(statusState);

  const { mainNavLinks } = useNavigation(t, docsLink, headerNavModules);

  const [openMenu, setOpenMenu] = useState(null);
  const themeMenuRef = useRef(null);
  const languageMenuRef = useRef(null);
  const userMenuRef = useRef(null);

  useEffect(() => {
    if (!openMenu) {
      return undefined;
    }

    const handlePointerDown = (event) => {
      const refs = [themeMenuRef, languageMenuRef, userMenuRef];
      const clickedInsideMenu = refs.some(
        (ref) => ref.current && ref.current.contains(event.target),
      );
      if (!clickedInsideMenu) {
        setOpenMenu(null);
      }
    };

    document.addEventListener('mousedown', handlePointerDown);
    return () => {
      document.removeEventListener('mousedown', handlePointerDown);
    };
  }, [openMenu]);

  const themeOptions = useMemo(
    () => [
      { key: 'light', icon: Sun, label: t('浅色模式') },
      { key: 'dark', icon: Moon, label: t('深色模式') },
      { key: 'auto', icon: Monitor, label: t('自动模式') },
    ],
    [t],
  );

  const languageOptions = useMemo(
    () => [
      ['zh-CN', '简体中文'],
      ['zh-TW', '繁體中文'],
      ['en', 'English'],
      ['fr', 'Français'],
      ['ja', '日本語'],
      ['ru', 'Русский'],
      ['vi', 'Tiếng Việt'],
    ],
    [],
  );

  const currentThemeIcon = themeIconMap[theme] || Monitor;

  const renderNavLinks = () =>
    mainNavLinks.map((link) => {
      const linkLabel = <span>{link.text}</span>;

      if (link.isExternal) {
        return (
          <a
            key={link.itemKey}
            href={link.externalLink}
            target='_blank'
            rel='noopener noreferrer'
            className='flex-shrink-0 rounded-lg px-3 py-2 text-lg font-semibold text-slate-800 transition hover:text-slate-950 dark:text-slate-100'
          >
            {linkLabel}
          </a>
        );
      }

      let targetPath = link.to;
      if (link.itemKey === 'console' && !userState.user) {
        targetPath = '/login';
      }
      if (link.itemKey === 'pricing' && pricingRequireAuth && !userState.user) {
        targetPath = '/login';
      }

      return (
        <Link
          key={link.itemKey}
          to={targetPath}
          className='flex-shrink-0 rounded-lg px-3 py-2 text-lg font-semibold text-slate-800 transition hover:text-slate-950 dark:text-slate-100'
        >
          {linkLabel}
        </Link>
      );
    });

  const renderUserControls = () => {
    if (isLoading) {
      return (
        <div className='h-10 w-20 animate-pulse rounded-full bg-slate-200/80 dark:bg-slate-700/70' />
      );
    }

    if (!userState.user) {
      return (
        <div className='flex items-center gap-2'>
          <Link
            to='/login'
            className='rounded-full border border-slate-200 bg-white/88 px-4 py-2 text-sm font-medium text-slate-700 shadow-sm transition hover:bg-white dark:border-slate-700 dark:bg-slate-900/88 dark:text-slate-200'
          >
            {t('登录')}
          </Link>
          {!isSelfUseMode ? (
            <Link
              to='/register'
              className='hidden rounded-full bg-slate-900 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-slate-800 md:inline-flex dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white'
            >
              {t('注册')}
            </Link>
          ) : null}
        </div>
      );
    }

    const username = userState.user.username || 'U';
    const hue = hashStringToHue(username);

    return (
      <div ref={userMenuRef} className='relative'>
        <button
          type='button'
          aria-label={username}
          aria-expanded={openMenu === 'user'}
          onClick={() => {
            setOpenMenu((current) => (current === 'user' ? null : 'user'));
          }}
          className='inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white/88 px-2 py-1.5 text-sm text-slate-700 shadow-sm transition hover:border-slate-300 hover:bg-white dark:border-slate-700 dark:bg-slate-900/88 dark:text-slate-200'
        >
          <span
            className='inline-flex h-8 w-8 items-center justify-center rounded-full text-sm font-semibold text-white'
            style={{ backgroundColor: `hsl(${hue} 62% 58%)` }}
          >
            {username[0].toUpperCase()}
          </span>
          <span className='hidden max-w-[88px] truncate font-medium md:inline'>
            {username}
          </span>
          <ChevronDown
            size={14}
            className='text-slate-500 dark:text-slate-400'
          />
        </button>
        <DropdownPanel open={openMenu === 'user'}>
          <MenuItem
            onClick={() => {
              setOpenMenu(null);
              navigate('/console/personal');
            }}
          >
            <Settings size={16} />
            <span>{t('个人设置')}</span>
          </MenuItem>
          <MenuItem
            onClick={() => {
              setOpenMenu(null);
              navigate('/console/token');
            }}
          >
            <KeyRound size={16} />
            <span>{t('令牌管理')}</span>
          </MenuItem>
          <MenuItem
            onClick={() => {
              setOpenMenu(null);
              navigate('/console/topup');
            }}
          >
            <CreditCard size={16} />
            <span>{t('钱包管理')}</span>
          </MenuItem>
          <div className='my-1 h-px bg-slate-200 dark:bg-slate-700' />
          <MenuItem
            destructive
            onClick={() => {
              setOpenMenu(null);
              logout();
            }}
          >
            <LogOut size={16} />
            <span>{t('退出')}</span>
          </MenuItem>
        </DropdownPanel>
      </div>
    );
  };

  return (
    <header className='sticky top-0 z-50 border-b border-slate-200/80 bg-white/78 text-slate-900 backdrop-blur-xl dark:border-slate-800/80 dark:bg-slate-950/78 dark:text-slate-100'>
      {noticeVisible ? (
        <Suspense fallback={null}>
          <MarketingNoticeModal
            visible={noticeVisible}
            onClose={handleNoticeClose}
            isMobile={isMobile}
            defaultTab={unreadCount > 0 ? 'system' : 'inApp'}
            unreadKeys={getUnreadKeys()}
          />
        </Suspense>
      ) : null}

      <div className='w-full px-2'>
        <div className='flex h-16 items-center justify-between gap-3'>
          <div className='flex min-w-0 items-center'>
            <HeaderLogo
              isMobile={isMobile}
              isConsoleRoute={false}
              logo={logo}
              isSelfUseMode={isSelfUseMode}
              isDemoSiteMode={isDemoSiteMode}
              t={t}
            />
          </div>

          <nav className='mx-2 hidden flex-1 items-center gap-1 overflow-x-auto whitespace-nowrap scrollbar-hide md:flex'>
            {renderNavLinks()}
          </nav>

          <div className='flex items-center gap-2'>
            <MenuButton
              ref={themeMenuRef}
              icon={currentThemeIcon}
              label={t('切换主题')}
              isOpen={openMenu === 'theme'}
              onClick={() => {
                setOpenMenu((current) =>
                  current === 'theme' ? null : 'theme',
                );
              }}
            >
              <DropdownPanel open={openMenu === 'theme'}>
                {themeOptions.map((option) => {
                  const Icon = option.icon;
                  return (
                    <MenuItem
                      key={option.key}
                      isSelected={theme === option.key}
                      onClick={() => {
                        handleThemeToggle(option.key);
                        setOpenMenu(null);
                      }}
                    >
                      <Icon size={16} />
                      <span>{option.label}</span>
                    </MenuItem>
                  );
                })}
              </DropdownPanel>
            </MenuButton>

            <MenuButton
              ref={languageMenuRef}
              icon={Languages}
              label={t('common.changeLanguage')}
              isOpen={openMenu === 'language'}
              onClick={() => {
                setOpenMenu((current) =>
                  current === 'language' ? null : 'language',
                );
              }}
            >
              <DropdownPanel open={openMenu === 'language'}>
                {languageOptions.map(([value, label]) => (
                  <MenuItem
                    key={value}
                    isSelected={currentLang === value}
                    onClick={() => {
                      handleLanguageChange(value);
                      setOpenMenu(null);
                    }}
                  >
                    <span>{label}</span>
                  </MenuItem>
                ))}
              </DropdownPanel>
            </MenuButton>

            <MenuButton
              icon={Bell}
              label={t('系统公告')}
              badgeCount={unreadCount}
              onClick={handleNoticeOpen}
            />

            {renderUserControls()}
          </div>
        </div>
      </div>
    </header>
  );
};

export default MarketingHeaderBar;
