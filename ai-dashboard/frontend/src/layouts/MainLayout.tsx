import { useEffect, useState } from 'react'
import { Layout, Menu, Dropdown, Avatar, Button, Space, Tooltip, Grid } from 'antd'
import {
  DashboardOutlined,
  ClusterOutlined,
  UserOutlined,
  TeamOutlined,
  DatabaseOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  GlobalOutlined,
  CloudServerOutlined,
  AppstoreOutlined,
  InboxOutlined,
  SettingOutlined,
  DollarOutlined,
  DeploymentUnitOutlined,
} from '@ant-design/icons'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useUserStore } from '@/store/user'
import { changeLanguage } from '@/i18n'
import BrandLogo from '@/components/BrandLogo'
import ErrorBoundary from '@/components/ErrorBoundary'
import type { MenuProps } from 'antd'

const { Header, Sider, Content } = Layout

export default function MainLayout() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const { user, logout } = useUserStore()
  const [collapsed, setCollapsed] = useState(false)
  const [scrolled, setScrolled] = useState(false)
  const screens = Grid.useBreakpoint()

  // Auto-collapse the sidebar on narrow screens for a usable mobile layout.
  useEffect(() => {
    if (screens.lg === false) setCollapsed(true)
    if (screens.lg === true) setCollapsed(false)
  }, [screens.lg])

  // Elevate the header once the content scrolls beneath it.
  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 4)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  const menuItems: MenuProps['items'] = [
    {
      type: 'group',
      label: collapsed ? null : <span className="menu-caption">{t('menu.sectionObserve')}</span>,
      children: [
        { key: '/dashboard', icon: <DashboardOutlined />, label: t('menu.dashboard') },
        { key: '/nodes', icon: <CloudServerOutlined />, label: t('menu.nodes') },
        { key: '/quota', icon: <ClusterOutlined />, label: t('menu.quota') },
        { key: '/workloads', icon: <AppstoreOutlined />, label: t('menu.workloads') },
        { key: '/cost', icon: <DollarOutlined />, label: t('menu.cost') },
        { key: '/models', icon: <DeploymentUnitOutlined />, label: t('menu.models') },
      ],
    },
    {
      type: 'group',
      label: collapsed ? null : <span className="menu-caption">{t('menu.sectionManage')}</span>,
      children: [
        { key: '/researchers', icon: <UserOutlined />, label: t('menu.researchers') },
        { key: '/user-groups', icon: <TeamOutlined />, label: t('menu.userGroups') },
        { key: '/datasets', icon: <DatabaseOutlined />, label: t('menu.datasets') },
      ],
    },
    {
      type: 'group',
      label: collapsed ? null : <span className="menu-caption">{t('menu.sectionSystem')}</span>,
      children: [
        { key: '/images', icon: <InboxOutlined />, label: t('menu.images') },
        { key: '/settings', icon: <SettingOutlined />, label: t('menu.settings') },
      ],
    },
  ]

  const currentPath = '/' + location.pathname.split('/')[1]

  const flatItems = (menuItems as Array<{ children?: Array<{ key?: string; label?: React.ReactNode }> }>)
    .flatMap((g) => g.children || [])
  const currentMenu = flatItems.find((m) => m.key === currentPath)

  const langItems: MenuProps['items'] = [
    { key: 'zh', label: '中文', onClick: () => changeLanguage('zh') },
    { key: 'en', label: 'English', onClick: () => changeLanguage('en') },
  ]

  const userItems: MenuProps['items'] = [
    {
      key: 'user',
      label: (
        <div style={{ padding: '4px 2px' }}>
          <div style={{ fontWeight: 600, color: 'var(--text-1)' }}>{user?.userName || 'admin'}</div>
          <div style={{ fontSize: 11, color: 'var(--text-3)' }}>{t('header.role')}</div>
        </div>
      ),
      disabled: true,
    },
    { type: 'divider' },
    { key: 'logout', icon: <LogoutOutlined />, label: t('header.logout'), danger: true, onClick: () => logout() },
  ]

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        trigger={null}
        collapsible
        collapsed={collapsed}
        width={248}
        collapsedWidth={72}
        theme="light"
        className="app-sider"
        style={{
          position: 'fixed',
          left: 0,
          top: 0,
          bottom: 0,
          overflow: 'auto',
          zIndex: 100,
          scrollbarWidth: 'none',
        }}
      >
        <div
          style={{
            height: 68,
            display: 'flex',
            alignItems: 'center',
            padding: collapsed ? 0 : '0 18px',
            justifyContent: collapsed ? 'center' : 'flex-start',
          }}
        >
          <BrandLogo
            size={34}
            collapsed={collapsed}
            title={t('brand')}
            subtitle={t('brandSub')}
          />
        </div>

        <Menu
          mode="inline"
          selectedKeys={[currentPath]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
          style={{ borderRight: 0, background: 'transparent' }}
        />

        <div
          style={{
            position: 'sticky',
            bottom: 0,
            padding: collapsed ? '14px 0' : '14px 18px',
            marginTop: 'auto',
            display: 'flex',
            justifyContent: 'center',
          }}
        >
          <Tooltip title={collapsed ? t('header.expand') : t('header.collapse')} placement="right">
            <Button
              type="text"
              aria-label={collapsed ? t('header.expand') : t('header.collapse')}
              icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
              onClick={() => setCollapsed(!collapsed)}
              style={{ color: 'var(--text-2)' }}
            />
          </Tooltip>
        </div>
      </Sider>

      <Layout
        style={{
          marginLeft: collapsed ? 72 : 248,
          transition: 'margin-left 0.28s cubic-bezier(0.22, 1, 0.36, 1)',
        }}
      >
        <Header
          className="glass"
          style={{
            padding: '0 28px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            height: 60,
            position: 'sticky',
            top: 0,
            zIndex: 99,
            borderBottom: '1px solid var(--border-2)',
            boxShadow: scrolled ? '0 8px 24px -12px rgba(29,29,31,0.12)' : 'none',
            transition: 'box-shadow 0.28s cubic-bezier(0.22, 1, 0.36, 1)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 0 }}>
            {screens.md && (
              <span style={{ fontSize: 15, fontWeight: 600, color: 'var(--text-1)', letterSpacing: '-0.01em' }}>
                {currentMenu?.label || t('brand')}
              </span>
            )}
          </div>

          <Space size={8}>
            <Dropdown menu={{ items: langItems }} placement="bottomRight">
              <Button
                type="text"
                aria-label={t('header.language')}
                icon={<GlobalOutlined />}
                style={{ fontSize: 13, color: 'var(--text-2)' }}
              >
                {i18n.language === 'zh' ? '中文' : 'EN'}
              </Button>
            </Dropdown>
            <Dropdown menu={{ items: userItems }} placement="bottomRight" trigger={['click']}>
              <div
                className="pressable"
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 9,
                  cursor: 'pointer',
                  padding: '5px 10px 5px 6px',
                  borderRadius: 999,
                  border: '1px solid var(--border-2)',
                  background: 'var(--bg-surface)',
                }}
              >
                <Avatar
                  size={26}
                  style={{
                    background: 'linear-gradient(135deg, #0071e3, #5856d6)',
                    fontSize: 12,
                    fontWeight: 600,
                  }}
                >
                  {user?.userName?.[0]?.toUpperCase() || 'A'}
                </Avatar>
                {screens.sm && (
                  <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-1)', maxWidth: 140, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {user?.userName || 'admin'}
                  </span>
                )}
              </div>
            </Dropdown>
          </Space>
        </Header>

        <Content style={{ margin: 0, minHeight: 'calc(100vh - 60px)' }}>
          <div
            style={{ maxWidth: 'var(--content-max)', margin: '0 auto', padding: '28px 28px 56px' }}
          >
            <ErrorBoundary>
              <div key={location.pathname} className="page-transition">
                <Outlet />
              </div>
            </ErrorBoundary>
          </div>
        </Content>
      </Layout>
    </Layout>
  )
}
