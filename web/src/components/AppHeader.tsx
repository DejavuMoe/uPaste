import React from 'react';
import { Link, useLocation } from 'react-router';
import { useTheme } from '../app/theme';
import { useLanguage } from '../app/locale';
import type { Theme } from '../app/types';

export const AppHeader: React.FC = () => {
  const { theme, setTheme } = useTheme();
  const { language, setLanguage } = useLanguage();
  const isCreate = useLocation().pathname === '/';

  if (isCreate) {
    return (
      <header className="app-header create-header">
        <div className="header-container">
          <Link to="/" className="brand-logo" aria-label="uPaste"><span className="brand-name">uPaste</span></Link>
          <div className="header-tools">
            <div className="locale-switch" role="group" aria-label="语言 / Language">
              <button type="button" aria-pressed={language === 'zh'} onClick={() => setLanguage('zh')}>中文</button>
              <span aria-hidden="true">/</span>
              <button type="button" aria-pressed={language === 'en'} onClick={() => setLanguage('en')}>EN</button>
            </div>
            <label htmlFor="theme-select" className="sr-only">{language === 'zh' ? '外观' : 'Theme'}</label>
            <select
              id="theme-select"
              className="form-select theme-select"
              value={theme}
              onChange={(event) => setTheme(event.target.value as Theme)}
              aria-label={language === 'zh' ? '外观' : 'Theme'}
            >
              <option value="system">{language === 'zh' ? '跟随系统' : 'System'}</option>
              <option value="light">{language === 'zh' ? '浅色' : 'Light'}</option>
              <option value="dark">{language === 'zh' ? '深色' : 'Dark'}</option>
            </select>
          </div>
        </div>
      </header>
    );
  }

  return (
    <header className="app-header">
      <div className="header-container">
        <Link to="/" className="brand-logo" aria-label="uPaste Home">
          <span className="brand-name">uPaste</span>
        </Link>

        <nav className="header-nav" aria-label="Application navigation">
          <div className="theme-selector-container">
            <label htmlFor="theme-select" className="sr-only">
              Color theme
            </label>
            <select
              id="theme-select"
              className="form-select theme-select"
              value={theme}
              onChange={(e) => setTheme(e.target.value as any)}
              aria-label="Color theme"
            >
              <option value="system">System theme</option>
              <option value="light">Light</option>
              <option value="dark">Dark</option>
            </select>
          </div>
        </nav>
      </div>
    </header>
  );
};
