import React from 'react';
import { createBrowserRouter, Navigate, Outlet, RouterProvider, type RouteObject } from 'react-router';
import { ThemeProvider } from './theme';
import { OwnerCapabilityProvider } from './ownerCapabilities';
import { ConfigProvider } from './config';
import { AppHeader } from '../components/AppHeader';
import { CreatePage } from '../features/create/CreatePage';
import { ShareRoute } from '../features/share/ShareRoute';
import { ManageRoute } from '../features/manage/ManageRoute';
import { AdminPage } from '../features/admin/AdminPage';

export const appRoutes: RouteObject[] = [
  {
    path: '/',
    element: <div className="app-layout"><AppHeader /><Outlet /></div>,
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
    <ConfigProvider>
      <OwnerCapabilityProvider>
        <RouterProvider router={router} />
      </OwnerCapabilityProvider>
    </ConfigProvider>
  </ThemeProvider>
);

export default App;
