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

import React, { useEffect, useState, useMemo, useContext } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { getFooterHTML, getLogo, getSystemName } from '../../helpers/branding';
import { StatusContext } from '../../context/Status';
import { getOptimizedLogoUrl } from '../../helpers/logo';
import BrandWordmark from '../common/BrandWordmark';

const FooterBar = () => {
  const { t } = useTranslation();
  const [footer, setFooter] = useState(getFooterHTML());
  const systemName = getSystemName();
  const logo = getLogo();
  const displayLogo = getOptimizedLogoUrl(logo, { size: 128 });
  const [statusState] = useContext(StatusContext);
  const isDemoSiteMode = statusState?.status?.demo_site_enabled || false;

  const loadFooter = () => {
    const footer_html = localStorage.getItem('footer_html');
    setFooter(footer_html || '');
  };

  const currentYear = new Date().getFullYear();

  const legalLinks = useMemo(
    () => (
      <div className='flex flex-wrap items-center justify-center gap-x-6 gap-y-2 text-sm text-semi-color-text-1'>
        <Link to='/about' className='!text-semi-color-text-1'>
          {t('关于')}
        </Link>
        <Link to='/user-agreement' className='!text-semi-color-text-1'>
          {t('服务条款')}
        </Link>
        <Link to='/privacy-policy' className='!text-semi-color-text-1'>
          {t('隐私政策')}
        </Link>
      </div>
    ),
    [t],
  );

  const copyrightLine = useMemo(
    () => (
      <span className='text-sm !text-semi-color-text-1'>
        © {currentYear} {systemName}. {t('版权所有')}
      </span>
    ),
    [currentYear, systemName, t],
  );

  const customFooter = useMemo(
    () => (
      <footer className='relative h-auto py-8 md:py-10 px-6 md:px-24 w-full flex flex-col items-center overflow-hidden'>
        {isDemoSiteMode && (
          <div className='flex flex-col md:flex-row justify-between w-full max-w-[1110px] mb-10 gap-8'>
            <div className='flex-shrink-0'>
              {displayLogo ? (
                <img
                  src={displayLogo}
                  alt={systemName}
                  width='64'
                  height='64'
                  decoding='async'
                  className='w-16 h-16 rounded-full bg-gray-800 p-1.5 object-contain'
                />
              ) : (
                <BrandWordmark variant='header' className='text-base' />
              )}
            </div>

            <div className='grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-8 w-full'>
              <div className='text-left'>
                <p className='!text-semi-color-text-0 font-semibold mb-5'>
                  {t('相关项目')}
                </p>
                <div className='flex flex-col gap-4'>
                  <a
                    href='https://github.com/songquanpeng/one-api'
                    target='_blank'
                    rel='noopener noreferrer'
                    className='!text-semi-color-text-1'
                  >
                    One API
                  </a>
                  <a
                    href='https://github.com/novicezk/midjourney-proxy'
                    target='_blank'
                    rel='noopener noreferrer'
                    className='!text-semi-color-text-1'
                  >
                    Midjourney-Proxy
                  </a>
                  <a
                    href='https://github.com/Calcium-Ion/new-api-key-tool'
                    target='_blank'
                    rel='noopener noreferrer'
                    className='!text-semi-color-text-1'
                  >
                    new-api-key-tool
                  </a>
                </div>
              </div>

              <div className='text-left'>
                <p className='!text-semi-color-text-0 font-semibold mb-5'>
                  {t('友情链接')}
                </p>
                <div className='flex flex-col gap-4'>
                  <a
                    href='https://github.com/ca0fgh/hermestoken-horizon'
                    target='_blank'
                    rel='noopener noreferrer'
                    className='!text-semi-color-text-1'
                  >
                    hermestoken-horizon
                  </a>
                  <a
                    href='https://github.com/coaidev/coai'
                    target='_blank'
                    rel='noopener noreferrer'
                    className='!text-semi-color-text-1'
                  >
                    CoAI
                  </a>
                  <a
                    href='https://www.gpt-load.com/'
                    target='_blank'
                    rel='noopener noreferrer'
                    className='!text-semi-color-text-1'
                  >
                    GPT-Load
                  </a>
                </div>
              </div>
            </div>
          </div>
        )}

        <div className='flex w-full max-w-[1110px] flex-col items-center gap-3 text-center'>
          {legalLinks}
          {copyrightLine}
        </div>
      </footer>
    ),
    [displayLogo, systemName, t, isDemoSiteMode, legalLinks, copyrightLine],
  );

  useEffect(() => {
    loadFooter();
  }, [statusState?.status?.footer_html]);

  return (
    <div className='w-full'>
      {footer ? (
        <footer className='relative h-auto py-6 px-6 md:px-24 w-full flex items-center justify-center overflow-hidden'>
          <div className='flex w-full max-w-[1110px] flex-col items-center gap-3 text-center'>
            <div
              className='custom-footer na-cb6feafeb3990c78 text-sm !text-semi-color-text-1'
              dangerouslySetInnerHTML={{ __html: footer }}
            ></div>
            {legalLinks}
            {copyrightLine}
          </div>
        </footer>
      ) : (
        customFooter
      )}
    </div>
  );
};

export default FooterBar;
