import React, { useEffect, useState } from 'react';
import { Card, Row, Col, Typography } from 'antd';
import {
  CodeOutlined,
  ThunderboltOutlined,
  CloudServerOutlined,
  RocketOutlined,
  ExperimentOutlined,
  ArrowRightOutlined,
  InboxOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { getOverview, DashboardOverview } from '../api/user';
import { useUserStore } from '../store/user';
import CountUp from '../components/CountUp';
import EmptyState from '../components/EmptyState';

const { Text } = Typography;

function greetingKey(): string {
  const h = new Date().getHours();
  if (h < 6) return 'dashboard.greeting.night';
  if (h < 12) return 'dashboard.greeting.morning';
  if (h < 18) return 'dashboard.greeting.afternoon';
  return 'dashboard.greeting.evening';
}

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

  const statCards = [
    {
      title: t('dashboard.notebooks'),
      icon: <CodeOutlined style={{ fontSize: 18 }} />,
      data: overview?.notebooks,
      color: '#6366f1',
      path: '/notebooks',
    },
    {
      title: t('dashboard.training'),
      icon: <ThunderboltOutlined style={{ fontSize: 18 }} />,
      data: overview?.trainingJobs,
      color: '#a855f7',
      path: '/training',
    },
    {
      title: t('dashboard.serving'),
      icon: <CloudServerOutlined style={{ fontSize: 18 }} />,
      data: overview?.servingJobs,
      color: '#0ea5e9',
      path: '/serving',
    },
  ];

  const quickActions = [
    { icon: <CodeOutlined />, label: t('dashboard.quickstart.notebook'), path: '/notebooks', color: '#6366f1' },
    { icon: <ThunderboltOutlined />, label: t('dashboard.quickstart.training'), path: '/training', color: '#a855f7' },
    { icon: <RocketOutlined />, label: t('dashboard.quickstart.deploy'), path: '/serving', color: '#0ea5e9' },
    { icon: <ExperimentOutlined />, label: t('dashboard.quickstart.models'), path: '/models', color: '#d946ef' },
  ];

  const runningTotal =
    (overview?.notebooks?.running ?? 0) + (overview?.trainingJobs?.running ?? 0) + (overview?.servingJobs?.running ?? 0);
  const pendingTotal =
    (overview?.notebooks?.pending ?? 0) + (overview?.trainingJobs?.pending ?? 0) + (overview?.servingJobs?.pending ?? 0);
  const failedTotal =
    (overview?.notebooks?.failed ?? 0) + (overview?.trainingJobs?.failed ?? 0) + (overview?.servingJobs?.failed ?? 0);

  return (
    <div>
      {/* Welcome header */}
      <div className="anim-fade-up" style={{ marginBottom: 28 }}>
        <h1 style={{ margin: 0, fontSize: 24, fontWeight: 700, letterSpacing: '-0.02em', color: 'var(--text-1)' }}>
          {t(greetingKey())}, {user?.loginName || user?.name || 'Developer'}
        </h1>
        <p style={{ margin: '6px 0 0', fontSize: 13, color: 'var(--text-2)' }}>{t('dashboard.overview')}</p>
      </div>

      {/* Stat cards */}
      <Row gutter={[20, 20]} style={{ marginBottom: 24 }} className="stagger">
        {statCards.map((card, i) => (
          <Col xs={24} md={8} key={card.title} style={{ ['--i' as string]: i }}>
            <Card
              className="hover-lift pressable"
              bordered={false}
              style={{ borderRadius: 16, cursor: 'pointer', height: '100%' }}
              onClick={() => navigate(card.path)}
            >
              {loading ? (
                <div className="skeleton-block" style={{ height: 96, borderRadius: 12 }} />
              ) : (
                <>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 18 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                      <div
                        style={{
                          width: 34,
                          height: 34,
                          borderRadius: 10,
                          background: `${card.color}14`,
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          color: card.color,
                        }}
                      >
                        {card.icon}
                      </div>
                      <Text strong style={{ fontSize: 14 }}>
                        {card.title}
                      </Text>
                    </div>
                    <ArrowRightOutlined style={{ color: card.color, opacity: 0.55, fontSize: 13 }} />
                  </div>
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
                    <span style={{ fontSize: 34, fontWeight: 700, letterSpacing: '-0.02em', color: 'var(--text-1)', lineHeight: 1 }}>
                      <CountUp value={card.data?.total ?? 0} />
                    </span>
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {t('dashboard.total')}
                    </Text>
                  </div>
                  <div style={{ marginTop: 10, display: 'flex', gap: 14 }}>
                    <span className="status-pill ok">
                      <span className="dot" />
                      <CountUp value={card.data?.running ?? 0} /> {t('dashboard.running')}
                    </span>
                    {(card.data?.pending ?? 0) > 0 && (
                      <span className="status-pill warn">
                        <span className="dot" />
                        <CountUp value={card.data?.pending ?? 0} /> {t('dashboard.pending')}
                      </span>
                    )}
                    {(card.data?.failed ?? 0) > 0 && (
                      <span className="status-pill err">
                        <span className="dot" />
                        <CountUp value={card.data?.failed ?? 0} /> {t('dashboard.failed')}
                      </span>
                    )}
                  </div>
                </>
              )}
            </Card>
          </Col>
        ))}
      </Row>

      {/* Quick start */}
      <Card
        title={<Text strong>{t('dashboard.quickstart')}</Text>}
        bordered={false}
        style={{ borderRadius: 16, marginBottom: 24 }}
        className="anim-fade-up"
      >
        <Row gutter={[16, 16]} className="stagger">
          {quickActions.map((action, i) => (
            <Col xs={24} sm={12} md={6} key={action.label} style={{ ['--i' as string]: i }}>
              <div
                role="button"
                tabIndex={0}
                className="pressable"
                onClick={() => navigate(action.path)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') navigate(action.path);
                }}
                style={{
                  height: 72,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: 10,
                  borderRadius: 12,
                  border: '1px dashed var(--border-1)',
                  cursor: 'pointer',
                  background: 'var(--bg-sunken)',
                  transition: 'border-color 0.16s cubic-bezier(0.22,1,0.36,1), background 0.16s cubic-bezier(0.22,1,0.36,1)',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.borderColor = action.color;
                  e.currentTarget.style.background = `${action.color}0a`;
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.borderColor = 'var(--border-1)';
                  e.currentTarget.style.background = 'var(--bg-sunken)';
                }}
              >
                <span style={{ color: action.color, fontSize: 16 }}>{action.icon}</span>
                <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-1)' }}>{action.label}</span>
              </div>
            </Col>
          ))}
        </Row>
      </Card>

      {/* Platform info */}
      <Row gutter={[20, 20]} className="stagger">
        <Col xs={24} md={12} style={{ ['--i' as string]: 0 }}>
          <Card bordered={false} style={{ borderRadius: 16, height: '100%' }}>
            <h3 style={{ margin: '0 0 4px', fontSize: 15, fontWeight: 600, color: 'var(--text-1)' }}>
              {t('dashboard.status.title')}
            </h3>
            <p style={{ margin: '0 0 20px', fontSize: 12, color: 'var(--text-2)' }}>{t('dashboard.status.desc')}</p>
            <Row gutter={16}>
              <Col span={8}>
                <div style={{ textAlign: 'center' }}>
                  <div className="tnum" style={{ fontSize: 26, fontWeight: 700, color: 'var(--ok)' }}>
                    <CountUp value={runningTotal} />
                  </div>
                  <div style={{ fontSize: 12, color: 'var(--text-2)', marginTop: 2 }}>{t('dashboard.running')}</div>
                </div>
              </Col>
              <Col span={8}>
                <div style={{ textAlign: 'center' }}>
                  <div className="tnum" style={{ fontSize: 26, fontWeight: 700, color: 'var(--warn)' }}>
                    <CountUp value={pendingTotal} />
                  </div>
                  <div style={{ fontSize: 12, color: 'var(--text-2)', marginTop: 2 }}>{t('dashboard.pending')}</div>
                </div>
              </Col>
              <Col span={8}>
                <div style={{ textAlign: 'center' }}>
                  <div className="tnum" style={{ fontSize: 26, fontWeight: 700, color: 'var(--err)' }}>
                    <CountUp value={failedTotal} />
                  </div>
                  <div style={{ fontSize: 12, color: 'var(--text-2)', marginTop: 2 }}>{t('dashboard.failed')}</div>
                </div>
              </Col>
            </Row>
          </Card>
        </Col>
        <Col xs={24} md={12} style={{ ['--i' as string]: 1 }}>
          <Card bordered={false} style={{ borderRadius: 16, height: '100%' }}>
            <h3 style={{ margin: '0 0 4px', fontSize: 15, fontWeight: 600, color: 'var(--text-1)' }}>
              {t('dashboard.recent')}
            </h3>
            <EmptyState
              icon={<InboxOutlined />}
              title={t('common.nodata')}
              description={t('dashboard.recentEmpty')}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
