import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router';
import { ThemeProvider } from './theme';
import { OwnerCapabilityProvider } from './ownerCapabilities';
import { AppHeader } from '../components/AppHeader';
import { CreatePage } from '../features/create/CreatePage';
import { ShareRoute } from '../features/share/ShareRoute';
import { ManageRoute } from '../features/manage/ManageRoute';

export const AppRoutes: React.FC = () => {
  return (
    <Routes>
      <Route path="/" element={<CreatePage />} />
      <Route path="/s/:id" element={<ShareRoute />} />
      <Route path="/manage/:id" element={<ManageRoute />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
};

export const App: React.FC = () => {
  return (
    <ThemeProvider>
      <OwnerCapabilityProvider>
        <BrowserRouter>
          <div className="app-layout">
            <AppHeader />
            <AppRoutes />
          </div>
        </BrowserRouter>
      </OwnerCapabilityProvider>
    </ThemeProvider>
  );
};

export default App;
