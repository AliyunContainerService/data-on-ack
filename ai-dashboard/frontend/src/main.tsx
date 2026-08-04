import React from 'react'
import ReactDOM from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import { ConfigProvider, App as AntApp } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import enUS from 'antd/locale/en_US'
import { router } from './router'
import './i18n'
import { getCurrentLanguage } from './i18n'

const lang = getCurrentLanguage()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ConfigProvider
      locale={lang === 'zh' ? zhCN : enUS}
      theme={{
        token: {
          colorPrimary: '#0071e3',
          colorBgContainer: '#ffffff',
          colorBgLayout: '#f5f5f7',
          colorBorder: '#e8e8ed',
          colorBorderSecondary: '#f0f0f3',
          borderRadius: 12,
          borderRadiusLG: 14,
          borderRadiusSM: 8,
          fontSize: 14,
          fontFamily: "-apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Helvetica Neue', Arial, sans-serif",
          colorText: '#1d1d1f',
          colorTextSecondary: '#6e6e73',
          controlHeight: 36,
          lineWidth: 1,
          boxShadow: '0 2px 8px rgba(0,0,0,0.04)',
          boxShadowSecondary: '0 4px 12px rgba(0,0,0,0.06)',
        },
        components: {
          Menu: {
            itemBorderRadius: 10,
            itemHeight: 40,
            itemMarginBlock: 4,
            itemMarginInline: 8,
          },
          Card: {
            borderRadiusLG: 14,
            paddingLG: 24,
          },
          Table: {
            borderRadiusLG: 12,
            headerBg: '#fafafa',
            headerBorderRadius: 12,
          },
          Button: {
            borderRadius: 10,
            controlHeight: 36,
          },
          Modal: {
            borderRadiusLG: 16,
          },
          Tag: {
            borderRadiusSM: 6,
          },
          Input: {
            borderRadius: 10,
          },
          Select: {
            borderRadius: 10,
          },
        },
      }}
    >
      <AntApp>
        <RouterProvider router={router} />
      </AntApp>
    </ConfigProvider>
  </React.StrictMode>,
)
