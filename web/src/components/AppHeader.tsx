import React from 'react';
import { Link } from 'react-router';
import { useTheme } from '../app/theme';

export const AppHeader: React.FC = () => {
  const { theme, setTheme } = useTheme();

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
