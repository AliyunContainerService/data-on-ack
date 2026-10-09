import type { ThemeConfig } from 'antd'

/**
 * Shared antd theme — aligned with the design tokens in styles/global.css.
 * Keep colors/radii in sync with the CSS custom properties.
 */
export const appTheme: ThemeConfig = {
  token: {
    colorPrimary: '#0071e3',
    colorInfo: '#0071e3',
    colorSuccess: '#34c759',
    colorWarning: '#ff9f0a',
    colorError: '#ff3b30',
    colorBgContainer: '#ffffff',
    colorBgLayout: '#f5f5f7',
    colorBorder: '#e8e8ed',
    colorBorderSecondary: '#f0f0f3',
    borderRadius: 12,
    borderRadiusLG: 14,
    borderRadiusSM: 8,
    fontSize: 14,
    fontFamily:
      "-apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Helvetica Neue', 'PingFang SC', 'Microsoft YaHei', Arial, sans-serif",
    colorText: '#1d1d1f',
    colorTextSecondary: '#6e6e73',
    colorTextTertiary: '#a1a1a6',
    controlHeight: 36,
    lineWidth: 1,
    boxShadow: '0 1px 2px rgba(29,29,31,0.04), 0 2px 6px rgba(29,29,31,0.05)',
    boxShadowSecondary: '0 4px 14px -2px rgba(29,29,31,0.08), 0 2px 4px -2px rgba(29,29,31,0.04)',
    motionDurationMid: '0.24s',
    motionEaseOut: 'cubic-bezier(0.22, 1, 0.36, 1)',
  },
  components: {
    Layout: {
      headerBg: 'rgba(255,255,255,0.72)',
      headerHeight: 60,
      headerPadding: '0 28px',
      siderBg: '#fbfbfd',
    },
    Menu: {
      itemBorderRadius: 10,
      itemHeight: 42,
      itemMarginBlock: 4,
      itemMarginInline: 10,
      itemSelectedBg: 'rgba(0,113,227,0.08)',
      itemSelectedColor: '#0071e3',
      itemColor: '#4b4b50',
      iconSize: 16,
    },
    Card: {
      borderRadiusLG: 16,
      paddingLG: 24,
    },
    Table: {
      borderRadiusLG: 12,
      headerBg: '#fafafc',
      headerBorderRadius: 12,
      headerSplitColor: 'transparent',
      rowHoverBg: 'rgba(0,113,227,0.035)',
    },
    Button: {
      borderRadius: 10,
      controlHeight: 36,
      primaryShadow: '0 8px 22px -6px rgba(0,113,227,0.38)',
    },
    Modal: {
      borderRadiusLG: 18,
    },
    Drawer: {
      borderRadiusLG: 18,
    },
    Tag: {
      borderRadiusSM: 6,
    },
    Input: {
      borderRadius: 10,
      activeShadow: '0 0 0 3px rgba(0,113,227,0.12)',
    },
    Select: {
      borderRadius: 10,
    },
    Tabs: {
      horizontalItemGutter: 28,
    },
    Progress: {
      borderRadius: 100,
    },
    Segmented: {
      borderRadius: 10,
      itemSelectedBg: '#ffffff',
    },
    Dropdown: {
      borderRadiusLG: 14,
    },
  },
}
