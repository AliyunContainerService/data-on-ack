import React, { useEffect, useState } from 'react';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { Layout, Menu, Dropdown, Button, Space, Avatar, Tooltip, Grid } from 'antd';
import type { MenuProps } from 'antd';
import {
  DashboardOutlined,
  CodeOutlined,
  ThunderboltOutlined,
  CloudServerOutlined,
  AppstoreOutlined,
  DatabaseOutlined,
  ExperimentOutlined,
  SlidersOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  TranslationOutlined,
  LogoutOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useUserStore } from '../store/user';
import BrandLogo from '../components/BrandLogo';
import PageLoading from '../components/PageLoading';
import ErrorBoundary from '../components/ErrorBoundary';

const { Header, Sider, Content } = Layout;

const MainLayout: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [collapsed, setCollapsed] = useState(false);
  const [scrolled, setScrolled] = useState(false);
  const { user, fetchUser, logout, locale, setLocale } = useUserStore();
  const screens = Grid.useBreakpoint();

  // Auto-collapse the sidebar on narrow screens for a usable mobile layout.
  useEffect(() => {
    if (screens.lg === false) setCollapsed(true);
    if (screens.lg === true) setCollapsed(false);
  }, [screens.lg]);

  // Elevate the header once the content scrolls beneath it.
  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 4);
    onScroll();
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
  }, []);

  // Block rendering of protected content until the session has been verified.
  // Previously a 1.5s timer redirected after the content was already shown; now we
  // render a loading placeholder first and only navigate to /login if the fetch fails.
  const [authChecked, setAuthChecked] = useState(() => useUserStore.getState().user !== null);

  useEffect(() => {
    if (authChecked) return;
    let cancelled = false;
    fetchUser().then(() => {
      if (cancelled) return;
      if (useUserStore.getState().user) {
        setAuthChecked(true);
      } else {
        navigate('/login', { replace: true });
      }
    });
    return () => {
      cancelled = true;
    };
  }, [authChecked, fetchUser, navigate]);

  if (!authChecked || !user) {
    return <PageLoading fullScreen />;
  }

  const menuItems: MenuProps['items'] = [
    {
      type: 'group',
      label: collapsed ? null : <span className="menu-caption">{t('menu.sectionWorkspace')}</span>,
      children: [
        { key: '/', icon: <DashboardOutlined />, label: t('menu.dashboard') },
        { key: '/notebooks', icon: <CodeOutlined />, label: t('menu.notebooks') },
        { key: '/training', icon: <ThunderboltOutlined />, label: t('menu.training') },
        { key: '/experiments', icon: <ExperimentOutlined />, label: t('menu.experiments') },
        { key: '/finetune', icon: <SlidersOutlined />, label: t('menu.finetune') },
        { key: '/serving', icon: <CloudServerOutlined />, label: t('menu.serving') },
      ],
    },
    {
      type: 'group',
      label: collapsed ? null : <span className="menu-caption">{t('menu.sectionAssets')}</span>,
      children: [
        { key: '/models', icon: <AppstoreOutlined />, label: t('menu.models') },
        { key: '/datasets', icon: <DatabaseOutlined />, label: t('menu.datasets') },
      ],
    },
  ];

  const handleMenuClick: MenuProps['onClick'] = ({ key }) => {
    navigate(key);
  };

  const handleLogout = async () => {
    try {
      await fetch('/api/v1/logout', { method: 'POST', credentials: 'include' });
    } catch {
      /* ignore */
    }
    logout();
    navigate('/login');
  };

  const userMenuItems: MenuProps['items'] = [
    {
      key: 'user',
      label: (
        <div style={{ padding: '4px 2px' }}>
          <div style={{ fontWeight: 600, color: 'var(--text-1)' }}>{user.loginName || user.name}</div>
          <div style={{ fontSize: 11, color: 'var(--text-3)' }}>{t('header.role')}</div>
        </div>
      ),
      disabled: true,
    },
    { type: 'divider' },
    { key: 'logout', icon: <LogoutOutlined />, label: t('user.logout'), danger: true, onClick: handleLogout },
  ];

  const toggleLocale = () => {
    setLocale(locale === 'zh' ? 'en' : 'zh');
  };

  const selectedKey = location.pathname === '/' ? '/' : '/' + location.pathname.split('/')[1];
  const flatItems = (menuItems as Array<{ children?: Array<{ key?: string; label?: React.ReactNode }> }>).flatMap(
    (g) => g.children || []
  );
  const currentMenu = flatItems.find((m) => m.key === selectedKey);

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
          <BrandLogo size={34} collapsed={collapsed} title={t('app.title')} subtitle={t('app.subtitle')} />
        </div>

        <Menu
          mode="inline"
          selectedKeys={[selectedKey]}
          items={menuItems}
          onClick={handleMenuClick}
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

      <Layout style={{ marginLeft: collapsed ? 72 : 248, transition: 'margin-left 0.28s cubic-bezier(0.22, 1, 0.36, 1)' }}>
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
                {currentMenu?.label || t('app.title')}
              </span>
            )}
          </div>
          <Space size={8} align="center">
            <Tooltip title={locale === 'zh' ? 'English' : '中文'}>
              <Button
                type="text"
                aria-label="Switch language"
                icon={<TranslationOutlined />}
                onClick={toggleLocale}
                style={{ fontSize: 13, color: 'var(--text-2)' }}
              >
                {locale === 'zh' ? '中文' : 'EN'}
              </Button>
            </Tooltip>
            <Dropdown menu={{ items: userMenuItems }} placement="bottomRight" trigger={['click']}>
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
                    background: 'linear-gradient(135deg, #6366f1, #a855f7)',
                    fontSize: 12,
                    fontWeight: 600,
                  }}
                >
                  {(user.loginName || user.name || 'A')[0].toUpperCase()}
                </Avatar>
                {screens.sm && (
                  <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-1)', maxWidth: 140, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {user.loginName || user.name}
                  </span>
                )}
              </div>
            </Dropdown>
          </Space>
        </Header>

        <Content style={{ margin: 0, minHeight: 'calc(100vh - 60px)' }}>
          <div style={{ maxWidth: 'var(--content-max)', margin: '0 auto', padding: '28px 28px 56px' }}>
            <ErrorBoundary>
              <div key={location.pathname} className="page-transition">
                <Outlet />
              </div>
            </ErrorBoundary>
          </div>
        </Content>
      </Layout>
    </Layout>
  );
};

export default MainLayout;
