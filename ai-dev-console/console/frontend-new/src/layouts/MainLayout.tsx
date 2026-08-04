import React, { useEffect, useState } from 'react';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { Layout, Menu, Dropdown, Button, Space, Avatar, Typography, Breadcrumb, Tooltip } from 'antd';
import type { MenuProps } from 'antd';
import {
  DashboardOutlined,
  CodeOutlined,
  ThunderboltOutlined,
  CloudServerOutlined,
  AppstoreOutlined,
  DatabaseOutlined,
  ExperimentOutlined,
  UserOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  TranslationOutlined,
  LogoutOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useUserStore } from '../store/user';

const { Header, Sider, Content } = Layout;
const { Text } = Typography;

const MainLayout: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [collapsed, setCollapsed] = useState(false);
  const { user, fetchUser, logout, locale, setLocale } = useUserStore();

  useEffect(() => {
    fetchUser();
  }, [fetchUser]);

  useEffect(() => {
    if (!user && !useUserStore.getState().loading) {
      const timer = setTimeout(() => {
        if (!useUserStore.getState().user && !useUserStore.getState().loading) {
          navigate('/login');
        }
      }, 1500);
      return () => clearTimeout(timer);
    }
  }, [user, navigate]);

  const menuItems: MenuProps['items'] = [
    { key: '/', icon: <DashboardOutlined />, label: t('menu.dashboard') },
    { key: '/notebooks', icon: <CodeOutlined />, label: t('menu.notebooks') },
    { key: '/training', icon: <ThunderboltOutlined />, label: t('menu.training') },
    { key: '/experiments', icon: <ExperimentOutlined />, label: t('menu.experiments') },
    { key: '/finetune', icon: <ExperimentOutlined />, label: t('menu.finetune') },
    { key: '/serving', icon: <CloudServerOutlined />, label: t('menu.serving') },
    { type: 'divider' },
    { key: '/models', icon: <AppstoreOutlined />, label: t('menu.models') },
    { key: '/datasets', icon: <DatabaseOutlined />, label: t('menu.datasets') },
  ];

  const handleMenuClick: MenuProps['onClick'] = ({ key }) => {
    navigate(key);
  };

  const handleLogout = async () => {
    try {
      await fetch('/api/v1/logout', { method: 'POST', credentials: 'include' });
    } catch { /* ignore */ }
    logout();
    navigate('/login');
  };

  const userMenuItems: MenuProps['items'] = [
    { key: 'settings', icon: <SettingOutlined />, label: t('user.settings') },
    { type: 'divider' },
    { key: 'logout', icon: <LogoutOutlined />, label: t('user.logout'), danger: true, onClick: handleLogout },
  ];

  const toggleLocale = () => {
    setLocale(locale === 'zh' ? 'en' : 'zh');
  };

  const pathSegments = location.pathname.split('/').filter(Boolean);
  const breadcrumbItems = [
    { title: t('app.title') },
    ...pathSegments.slice(0, 1).map((seg) => ({
      title: t(`menu.${seg}`) || seg,
    })),
  ];

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        trigger={null}
        collapsible
        collapsed={collapsed}
        width={240}
        collapsedWidth={68}
        theme="light"
        style={{
          position: 'fixed',
          left: 0,
          top: 0,
          bottom: 0,
          overflow: 'auto',
          zIndex: 100,
          borderRight: '1px solid #f0f0f3',
          background: '#fbfbfd',
        }}
      >
        <div style={{
          height: 60,
          display: 'flex',
          alignItems: 'center',
          justifyContent: collapsed ? 'center' : 'flex-start',
          padding: collapsed ? 0 : '0 24px',
        }}>
          {!collapsed && (
            <Text strong style={{ fontSize: 15, color: '#1d1d1f', letterSpacing: -0.3 }}>
              {t('app.title')}
            </Text>
          )}
          {collapsed && (
            <Text strong style={{ fontSize: 14, color: '#1d1d1f' }}>AI</Text>
          )}
        </div>

        <Menu
          mode="inline"
          selectedKeys={[location.pathname === '/' ? '/' : '/' + location.pathname.split('/')[1]]}
          items={menuItems}
          onClick={handleMenuClick}
          style={{ borderRight: 0, background: 'transparent', padding: '0 8px' }}
        />
      </Sider>

      <Layout style={{ marginLeft: collapsed ? 68 : 240, transition: 'margin-left 0.2s' }}>
        <Header style={{
          padding: '0 28px',
          background: '#ffffff',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          height: 56,
          position: 'sticky',
          top: 0,
          zIndex: 99,
          borderBottom: '1px solid #f0f0f3',
        }}>
          <Space size="middle">
            <Button
              type="text"
              icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
              onClick={() => setCollapsed(!collapsed)}
              style={{ fontSize: 16 }}
            />
            <Breadcrumb items={breadcrumbItems} />
          </Space>
          <Space size={12} align="center">
            <Tooltip title={locale === 'zh' ? 'English' : '中文'}>
              <Button
                type="text"
                icon={<TranslationOutlined />}
                onClick={toggleLocale}
                style={{ fontSize: 14 }}
              />
            </Tooltip>
            <Dropdown menu={{ items: userMenuItems }} placement="bottomRight" arrow>
              <div style={{ cursor: 'pointer', display: 'inline-flex', alignItems: 'center', gap: 8, lineHeight: 1 }}>
                <Avatar
                  size={28}
                  icon={<UserOutlined />}
                  style={{ background: '#0071e3', flexShrink: 0 }}
                />
                {user && (
                  <Text style={{ fontSize: 13, lineHeight: '28px', maxWidth: 120 }} ellipsis>
                    {user.loginName || user.name}
                  </Text>
                )}
              </div>
            </Dropdown>
          </Space>
        </Header>

        <Content style={{ margin: 0, padding: '24px 28px', background: '#f5f5f7', minHeight: 'calc(100vh - 56px)' }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
};

export default MainLayout;
