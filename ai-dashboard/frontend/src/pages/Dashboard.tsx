import { useState, useEffect } from 'react'
import { Tabs, Card, Row, Col, Typography, Table, Badge } from 'antd'
import {
  CloudServerOutlined,
  HddOutlined,
  ThunderboltOutlined,
  WarningOutlined,
  FundViewOutlined,
  InboxOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { get } from '@/api/client'
import PageHeader from '@/components/PageHeader'
import StatRing from '@/components/StatRing'
import CountUp from '@/components/CountUp'
import EmptyState from '@/components/EmptyState'

const { Text } = Typography

interface ClusterSummary {
  cpu: { capacity: number; allocated: number }
  memory: { capacity: number; allocated: number }
  gpu: { capacity: number; allocated: number }
  nodes: { total: number; ready: number; notReady: number; gpu: number }
}

interface EventInfo {
  namespace: string
  name: string
  kind: string
  reason: string
  message: string
  source: string
  count: number
  lastTimestamp: string
}

const pct = (used: number, total: number) => (total > 0 ? Math.round((used / total) * 100) : 0)

export default function Dashboard() {
  const { t } = useTranslation()
  const [summary, setSummary] = useState<ClusterSummary | null>(null)
  const [events, setEvents] = useState<EventInfo[]>([])
  const [loadingSummary, setLoadingSummary] = useState(true)
  const [loadingEvents, setLoadingEvents] = useState(true)

  useEffect(() => {
    get<ClusterSummary>('/ops/cluster-summary')
      .then((data: any) => setSummary(data))
      .catch(() => {})
      .finally(() => setLoadingSummary(false))

    get<EventInfo[]>('/ops/events')
      .then((data: any) => setEvents((data || []).slice(0, 20)))
      .catch(() => {})
      .finally(() => setLoadingEvents(false))
  }, [])

  const iframeStyle = { width: '100%', height: 'calc(100vh - 300px)', border: 'none', borderRadius: 12 }
  const buildUrl = (dashboardId: string) => `/grafana/d/${dashboardId}?orgId=1&refresh=10s&kiosk`

  const resources = summary
    ? [
        {
          label: t('dashboard.cpuUsage'),
          icon: <HddOutlined />,
          used: summary.cpu?.allocated ?? 0,
          total: summary.cpu?.capacity ?? 0,
          color: '#0071e3',
          unit: t('dashboard.unitCores'),
        },
        {
          label: t('dashboard.memUsage'),
          icon: <CloudServerOutlined />,
          used: summary.memory?.allocated ?? 0,
          total: summary.memory?.capacity ?? 0,
          color: '#5856d6',
          unit: 'GiB',
        },
        {
          label: t('dashboard.gpuUsage'),
          icon: <ThunderboltOutlined />,
          used: summary.gpu?.allocated ?? 0,
          total: summary.gpu?.capacity ?? 0,
          color: '#e8590c',
          unit: t('dashboard.unitCards'),
        },
      ]
    : []

  const eventColumns = [
    {
      title: t('event.time'),
      dataIndex: 'lastTimestamp',
      key: 'time',
      width: 170,
      render: (ts: string) => (
        <Text type="secondary" className="tnum" style={{ fontSize: 12 }}>
          {ts ? new Date(ts).toLocaleString() : '-'}
        </Text>
      ),
    },
    {
      title: t('event.reason'),
      dataIndex: 'reason',
      key: 'reason',
      width: 150,
      render: (r: string) => <span className="status-pill warn">{r || '-'}</span>,
    },
    { title: t('event.message'), dataIndex: 'message', key: 'message', ellipsis: true },
    {
      title: t('event.object'),
      key: 'object',
      width: 180,
      render: (_: unknown, r: EventInfo) => (
        <Text type="secondary" style={{ fontSize: 11, fontFamily: 'SF Mono, Menlo, monospace' }}>
          {r?.kind || ''}/{r?.name || ''}
        </Text>
      ),
    },
  ]

  const nodeVitals = summary
    ? [
        { label: t('dashboard.nodesTotal'), value: summary.nodes.total, cls: 'info' },
        { label: t('dashboard.nodesReady'), value: summary.nodes.ready, cls: 'ok' },
        { label: t('dashboard.nodesNotReady'), value: summary.nodes.notReady, cls: summary.nodes.notReady > 0 ? 'err' : 'idle' },
        { label: t('dashboard.gpuNodes'), value: summary.nodes.gpu, cls: 'info' },
      ]
    : []

  const overviewTab = (
    <div>
      {/* Cluster vitals */}
      {loadingSummary ? (
        <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
          {[0, 1, 2].map((i) => (
            <Col xs={24} md={8} key={i}>
              <div className="skeleton-block" style={{ height: 168, borderRadius: 16 }} />
            </Col>
          ))}
        </Row>
      ) : (
        <>
          <div
            className="anim-fade-up"
            style={{
              display: 'flex',
              gap: 10,
              flexWrap: 'wrap',
              marginBottom: 16,
              padding: '14px 18px',
              background: 'var(--bg-surface)',
              border: '1px solid var(--border-2)',
              borderRadius: 14,
              boxShadow: 'var(--shadow-xs)',
            }}
          >
            {nodeVitals.map((v) => (
              <span key={v.label} className={`status-pill ${v.cls}`}>
                <span className="dot" />
                {v.label}
                <strong className="tnum" style={{ marginLeft: 2 }}>
                  <CountUp value={v.value} />
                </strong>
              </span>
            ))}
          </div>

          <Row gutter={[16, 16]} style={{ marginBottom: 16 }} className="stagger">
            {resources.map((r, i) => (
              <Col xs={24} md={8} key={r.label} style={{ ['--i' as string]: i }}>
                <Card bordered={false} className="hover-lift" style={{ borderRadius: 16 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 18, flexWrap: 'wrap' }}>
                    <StatRing percent={pct(r.used, r.total)} color={r.color} size={108}>
                      <span className="tnum" style={{ fontSize: 22, fontWeight: 700, color: 'var(--text-1)', lineHeight: 1 }}>
                        {pct(r.used, r.total)}
                        <span style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-3)' }}>%</span>
                      </span>
                    </StatRing>
                    <div style={{ minWidth: 0 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
                        <span
                          style={{
                            width: 28,
                            height: 28,
                            borderRadius: 8,
                            display: 'inline-flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            color: r.color,
                            background: `${r.color}14`,
                            fontSize: 14,
                          }}
                        >
                          {r.icon}
                        </span>
                        <Text strong style={{ fontSize: 13 }}>
                          {r.label}
                        </Text>
                      </div>
                      <div style={{ fontSize: 12, color: 'var(--text-2)', lineHeight: 1.9 }}>
                        <div>
                          {t('dashboard.allocated')}
                          <strong className="tnum" style={{ color: 'var(--text-1)', marginLeft: 6 }}>
                            <CountUp value={r.used} />
                          </strong>
                          <span style={{ color: 'var(--text-3)', marginLeft: 4 }}>{r.unit}</span>
                        </div>
                        <div>
                          {t('dashboard.total')}
                          <span className="tnum" style={{ color: 'var(--text-1)', marginLeft: 6 }}>
                            <CountUp value={r.total} />
                          </span>
                          <span style={{ color: 'var(--text-3)', marginLeft: 4 }}>{r.unit}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </Card>
              </Col>
            ))}
          </Row>
        </>
      )}

      {/* Grafana Cluster Dashboard */}
      <Card
        bordered={false}
        style={{ borderRadius: 16, overflow: 'hidden' }}
        styles={{ body: { padding: 12 } }}
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <FundViewOutlined style={{ color: 'var(--accent)' }} />
            <span style={{ fontWeight: 600 }}>{t('dashboard.clusterMonitor')}</span>
            <span className="status-pill ok live">
              <span className="dot" />
              Live
            </span>
          </div>
        }
      >
        <iframe src={buildUrl('kube-ai-cluster-details')} style={iframeStyle} title="cluster-monitoring" />
      </Card>
    </div>
  )

  const eventsTab = (
    <Card bordered={false} style={{ borderRadius: 16 }}>
      <div style={{ marginBottom: 14, display: 'flex', alignItems: 'center', gap: 10 }}>
        <WarningOutlined style={{ color: 'var(--warn)' }} />
        <span style={{ fontWeight: 600, fontSize: 15 }}>{t('event.warnings')}</span>
        <Badge count={events.length} style={{ background: 'var(--warn)' }} />
      </div>
      <Table
        columns={eventColumns}
        dataSource={events}
        rowKey={(r, i) => `${r.namespace}/${r.name}/${i}`}
        pagination={{ pageSize: 15 }}
        size="middle"
        loading={loadingEvents}
        locale={{
          emptyText: <EmptyState icon={<InboxOutlined />} title={t('dashboard.noEvents')} description={t('dashboard.noEventsDesc')} />,
        }}
      />
    </Card>
  )

  const tabs = [
    { key: 'overview', label: t('dashboard.tabOverview'), children: overviewTab },
    {
      key: 'nodes',
      label: t('dashboard.nodes'),
      children: (
        <Card bordered={false} style={{ borderRadius: 16, overflow: 'hidden' }} styles={{ body: { padding: 12 } }}>
          <iframe src={buildUrl('kube-ai-node-details')} style={iframeStyle} title="node-monitoring" />
        </Card>
      ),
    },
    { key: 'events', label: t('dashboard.events'), children: eventsTab },
  ]

  return (
    <div>
      <PageHeader title={t('dashboard.cluster')} description={t('dashboard.heroDesc')} />
      <Tabs defaultActiveKey="overview" items={tabs} />
    </div>
  )
}
