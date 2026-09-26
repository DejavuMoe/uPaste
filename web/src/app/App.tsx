import React, { useEffect } from 'react';
import { createBrowserRouter, Navigate, Outlet, RouterProvider, useLocation, type RouteObject } from 'react-router';
import { ThemeProvider } from './theme';
import { LanguageProvider, useLanguage } from './locale';
import { OwnerCapabilityProvider } from './ownerCapabilities';
import { ConfigProvider } from './config';
import { AppHeader } from '../components/AppHeader';
import { CreatePage } from '../features/create/CreatePage';
import { ShareRoute } from '../features/share/ShareRoute';
import { ManageRoute } from '../features/manage/ManageRoute';
import { AdminPage } from '../features/admin/AdminPage';

const AppLayout: React.FC = () => {
  const isCreate = useLocation().pathname === '/';
  const { language } = useLanguage();
  useEffect(() => {
    document.documentElement.lang = isCreate && language === 'zh' ? 'zh-CN' : 'en';
  }, [isCreate, language]);
  return <div className={`app-layout${isCreate ? ' create-app' : ''}`}><AppHeader /><Outlet /></div>;
};

export const appRoutes: RouteObject[] = [
  {
    path: '/',
    element: <AppLayout />,
    children: [
      { index: true, element: <CreatePage /> },
      { path: 's/:id', element: <ShareRoute /> },
      { path: 'manage/:id', element: <ManageRoute /> },
      { path: 'admin', element: <AdminPage /> },
      { path: '*', element: <Navigate to="/" replace /> },
    ],
  },
];

const router = createBrowserRouter(appRoutes);

export const App: React.FC = () => (
  <ThemeProvider>
    <LanguageProvider>
      <ConfigProvider>
        <OwnerCapabilityProvider>
          <RouterProvider router={router} />
        </OwnerCapabilityProvider>
      </ConfigProvider>
    </LanguageProvider>
  </ThemeProvider>
);

export default App;
