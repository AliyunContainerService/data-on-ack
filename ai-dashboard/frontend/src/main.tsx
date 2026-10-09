import React from 'react'
import ReactDOM from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import { ConfigProvider, App as AntApp } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import enUS from 'antd/locale/en_US'
import { router } from './router'
import { appTheme } from './theme'
import ErrorBoundary from './components/ErrorBoundary'
import './i18n'
import { getCurrentLanguage } from './i18n'
import './styles/global.css'

const lang = getCurrentLanguage()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <ConfigProvider locale={lang === 'zh' ? zhCN : enUS} theme={appTheme}>
        <AntApp>
          <RouterProvider router={router} />
        </AntApp>
      </ConfigProvider>
    </ErrorBoundary>
  </React.StrictMode>,
)
