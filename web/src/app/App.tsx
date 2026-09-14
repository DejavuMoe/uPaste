import React from 'react';
import { createBrowserRouter, Navigate, Outlet, Route, RouterProvider, Routes } from 'react-router';
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

const router = createBrowserRouter([
  {
    path: '/',
    element: <div className="app-layout"><AppHeader /><Outlet /></div>,
    children: [
      { index: true, element: <CreatePage /> },
      { path: 's/:id', element: <ShareRoute /> },
      { path: 'manage/:id', element: <ManageRoute /> },
      { path: '*', element: <Navigate to="/" replace /> },
    ],
  },
]);

export const App: React.FC = () => (
  <ThemeProvider>
    <OwnerCapabilityProvider>
      <RouterProvider router={router} />
    </OwnerCapabilityProvider>
  </ThemeProvider>
);

export default App;
