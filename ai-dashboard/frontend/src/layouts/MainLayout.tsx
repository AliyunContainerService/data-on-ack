import { useState } from 'react'
import { Layout, Menu, Dropdown, Avatar, Button, Breadcrumb, Space, Typography } from 'antd'
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
import type { MenuProps } from 'antd'

const { Header, Sider, Content } = Layout
const { Text } = Typography

export default function MainLayout() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const { user, logout } = useUserStore()
  const [collapsed, setCollapsed] = useState(false)

  const menuItems: MenuProps['items'] = [
    { key: '/dashboard', icon: <DashboardOutlined />, label: t('menu.dashboard') },
    { key: '/nodes', icon: <CloudServerOutlined />, label: t('menu.nodes') },
    { key: '/quota', icon: <ClusterOutlined />, label: t('menu.quota') },
    { key: '/workloads', icon: <AppstoreOutlined />, label: t('menu.workloads') },
    { key: '/cost', icon: <DollarOutlined />, label: t('menu.cost') },
    { key: '/models', icon: <DeploymentUnitOutlined />, label: t('menu.models') },
    { type: 'divider' },
    { key: '/researchers', icon: <UserOutlined />, label: t('menu.researchers') },
    { key: '/user-groups', icon: <TeamOutlined />, label: t('menu.userGroups') },
    { key: '/datasets', icon: <DatabaseOutlined />, label: t('menu.datasets') },
    { type: 'divider' },
    { key: '/images', icon: <InboxOutlined />, label: t('menu.images') },
    { key: '/settings', icon: <SettingOutlined />, label: t('menu.settings') },
  ]

  const currentPath = '/' + location.pathname.split('/')[1]
  const currentMenu = (menuItems as Array<{ key?: string; label?: string }>).find(m => m.key === currentPath)

  const breadcrumbItems = [
    { title: t('brand') },
    ...(currentMenu?.label ? [{ title: currentMenu.label }] : []),
  ]

  const langItems: MenuProps['items'] = [
    { key: 'zh', label: '中文', onClick: () => changeLanguage('zh') },
    { key: 'en', label: 'English', onClick: () => changeLanguage('en') },
  ]

  const userItems: MenuProps['items'] = [
    { key: 'logout', icon: <LogoutOutlined />, label: t('header.logout'), danger: true, onClick: () => logout() },
  ]

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
        {/* Logo */}
        <div style={{
          height: 60,
          display: 'flex',
          alignItems: 'center',
          justifyContent: collapsed ? 'center' : 'flex-start',
          padding: collapsed ? 0 : '0 24px',
        }}>
          {!collapsed && (
            <Text strong style={{ fontSize: 15, color: '#1d1d1f', letterSpacing: -0.3 }}>
              {t('brand')}
            </Text>
          )}
          {collapsed && (
            <Text strong style={{ fontSize: 14, color: '#1d1d1f' }}>AI</Text>
          )}
        </div>

        <Menu
          mode="inline"
          selectedKeys={[currentPath]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
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

          <Space size={12}>
            <Dropdown menu={{ items: langItems }} placement="bottomRight">
              <Button type="text" icon={<GlobalOutlined />} style={{ fontSize: 14 }}>
                {i18n.language === 'zh' ? '中文' : 'EN'}
              </Button>
            </Dropdown>
            <Dropdown menu={{ items: userItems }} placement="bottomRight">
              <Space style={{ cursor: 'pointer' }} size={8}>
                <Avatar size={28} style={{ background: '#1677ff' }}>
                  {user?.userName?.[0]?.toUpperCase() || 'A'}
                </Avatar>
                <Text style={{ fontSize: 13 }}>
                  {user?.userName || 'admin'}
                </Text>
              </Space>
            </Dropdown>
          </Space>
        </Header>

        <Content style={{ margin: 0, padding: '24px 28px', background: '#f5f5f7', minHeight: 'calc(100vh - 56px)' }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
