import React from 'react';
import ReactDOM from 'react-dom/client';
import { RouterProvider } from 'react-router-dom';
import { ConfigProvider, App as AntApp } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import enUS from 'antd/locale/en_US';
import { router } from './router';
import { useUserStore } from './store/user';
import { appTheme } from './theme';
import ErrorBoundary from './components/ErrorBoundary';
import './i18n';
import './styles/global.css';

function Root() {
  const locale = useUserStore((s) => s.locale);
  const antdLocale = locale === 'zh' ? zhCN : enUS;

  return (
    <ConfigProvider locale={antdLocale} theme={appTheme}>
      <AntApp>
        <RouterProvider router={router} />
      </AntApp>
    </ConfigProvider>
  );
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <Root />
    </ErrorBoundary>
  </React.StrictMode>
);
