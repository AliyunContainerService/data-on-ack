import React, { useEffect, useState } from 'react';
import { Card, Row, Col, Statistic, Typography, Spin, Space, Button } from 'antd';
import {
  CodeOutlined,
  ThunderboltOutlined,
  CloudServerOutlined,
  RocketOutlined,
  ExperimentOutlined,
  PlusOutlined,
  ArrowRightOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { getOverview, DashboardOverview } from '../api/user';
import { useUserStore } from '../store/user';

const { Title, Text, Paragraph } = Typography;

const Dashboard: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const user = useUserStore((s) => s.user);
  const [overview, setOverview] = useState<DashboardOverview | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getOverview()
      .then(setOverview)
      .catch(() => setOverview(null))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return <Spin size="large" style={{ display: 'block', margin: '120px auto' }} />;
  }

  const statCards = [
    {
      title: t('dashboard.notebooks'),
      icon: <CodeOutlined style={{ fontSize: 20 }} />,
      data: overview?.notebooks,
      className: 'stat-card-blue',
      color: '#1677ff',
      path: '/notebooks',
    },
    {
      title: t('dashboard.training'),
      icon: <ThunderboltOutlined style={{ fontSize: 20 }} />,
      data: overview?.trainingJobs,
      className: 'stat-card-purple',
      color: '#722ed1',
      path: '/training',
    },
    {
      title: t('dashboard.serving'),
      icon: <CloudServerOutlined style={{ fontSize: 20 }} />,
      data: overview?.servingJobs,
      className: 'stat-card-green',
      color: '#13c2c2',
      path: '/serving',
    },
  ];

  const quickActions = [
    { icon: <CodeOutlined />, label: t('dashboard.quickstart.notebook'), path: '/notebooks', color: '#1677ff' },
    { icon: <ThunderboltOutlined />, label: t('dashboard.quickstart.training'), path: '/training', color: '#722ed1' },
    { icon: <RocketOutlined />, label: t('dashboard.quickstart.deploy'), path: '/serving', color: '#13c2c2' },
    { icon: <ExperimentOutlined />, label: t('dashboard.quickstart.models'), path: '/models', color: '#eb2f96' },
  ];

  return (
    <div>
      {/* Welcome header */}
      <div style={{ marginBottom: 28 }}>
        <Title level={4} style={{ marginBottom: 4 }}>
          {t('dashboard.welcome')}, {user?.loginName || user?.name || 'Developer'}
        </Title>
        <Text type="secondary">{t('dashboard.overview')}</Text>
      </div>

      {/* Stat cards */}
      <Row gutter={[20, 20]} style={{ marginBottom: 28 }}>
        {statCards.map((card) => (
          <Col xs={24} md={8} key={card.title}>
            <Card
              className={`hover-card ${card.className}`}
              bordered={false}
              style={{ borderRadius: 12, cursor: 'pointer' }}
              onClick={() => navigate(card.path)}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                <div>
                  <Space style={{ marginBottom: 16 }}>
                    <div style={{
                      width: 36, height: 36, borderRadius: 8,
                      background: `${card.color}15`,
                      display: 'flex', alignItems: 'center', justifyContent: 'center',
                      color: card.color,
                    }}>
                      {card.icon}
                    </div>
                    <Text strong style={{ fontSize: 15 }}>{card.title}</Text>
                  </Space>
                  <div style={{ display: 'flex', gap: 24 }}>
                    <Statistic
                      value={card.data?.total ?? 0}
                      suffix={<Text type="secondary" style={{ fontSize: 12 }}>total</Text>}
                      valueStyle={{ fontSize: 28, fontWeight: 600 }}
                    />
                  </div>
                  <div style={{ marginTop: 8, display: 'flex', gap: 16 }}>
                    <Text style={{ color: '#52c41a', fontSize: 12 }}>{card.data?.running ?? 0} running</Text>
                    <Text style={{ color: '#faad14', fontSize: 12 }}>{card.data?.pending ?? 0} pending</Text>
                    {(card.data?.failed ?? 0) > 0 && (
                      <Text style={{ color: '#ff4d4f', fontSize: 12 }}>{card.data?.failed} failed</Text>
                    )}
                  </div>
                </div>
                <ArrowRightOutlined style={{ color: card.color, opacity: 0.5 }} />
              </div>
            </Card>
          </Col>
        ))}
      </Row>

      {/* Quick start */}
      <Card
        title={<Text strong>{t('dashboard.quickstart')}</Text>}
        bordered={false}
        style={{ borderRadius: 12, marginBottom: 20 }}
      >
        <Row gutter={[16, 16]}>
          {quickActions.map((action) => (
            <Col xs={24} sm={12} md={6} key={action.label}>
              <Button
                block
                size="large"
                icon={<PlusOutlined />}
                onClick={() => navigate(action.path)}
                style={{
                  height: 64,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  borderRadius: 10,
                  borderStyle: 'dashed',
                  gap: 8,
                }}
              >
                <Space>
                  <span style={{ color: action.color }}>{action.icon}</span>
                  <span style={{ fontSize: 13 }}>{action.label}</span>
                </Space>
              </Button>
            </Col>
          ))}
        </Row>
      </Card>

      {/* Platform info */}
      <Row gutter={[20, 20]}>
        <Col xs={24} md={12}>
          <Card bordered={false} style={{ borderRadius: 12, height: '100%' }}>
            <Title level={5}>{t('dashboard.gpu.title')}</Title>
            <Paragraph type="secondary" style={{ marginBottom: 16 }}>
              GPU resource utilization across the cluster
            </Paragraph>
            <Row gutter={16}>
              <Col span={12}>
                <Statistic title={t('dashboard.gpu.used')} value={0} suffix="GPUs" valueStyle={{ color: '#1677ff' }} />
              </Col>
              <Col span={12}>
                <Statistic title={t('dashboard.gpu.total')} value={0} suffix="GPUs" />
              </Col>
            </Row>
          </Card>
        </Col>
        <Col xs={24} md={12}>
          <Card bordered={false} style={{ borderRadius: 12, height: '100%' }}>
            <Title level={5}>{t('dashboard.recent')}</Title>
            <Paragraph type="secondary">
              Your recent activities will appear here
            </Paragraph>
            <div style={{ color: '#9ca3af', textAlign: 'center', padding: '20px 0' }}>
              {t('common.nodata')}
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
