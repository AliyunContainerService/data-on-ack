import { useState, useEffect } from 'react'
import { Tabs, Card, Row, Col, Progress, Typography, Tag, Table, Spin, Space, Badge } from 'antd'
import { CloudServerOutlined, HddOutlined, ThunderboltOutlined, WarningOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { get } from '@/api/client'

const { Text, Title } = Typography

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

export default function Dashboard() {
  const { t } = useTranslation()
  const [summary, setSummary] = useState<ClusterSummary | null>(null)
  const [events, setEvents] = useState<EventInfo[]>([])
  const [loadingSummary, setLoadingSummary] = useState(true)

  useEffect(() => {
    get<ClusterSummary>('/ops/cluster-summary')
      .then((data: any) => setSummary(data))
      .catch(() => {})
      .finally(() => setLoadingSummary(false))

    get<EventInfo[]>('/ops/events')
      .then((data: any) => setEvents((data || []).slice(0, 20)))
      .catch(() => {})
  }, [])

  const iframeStyle = { width: '100%', height: 'calc(100vh - 280px)', border: 'none', borderRadius: 8 }
  const buildUrl = (dashboardId: string) => `/grafana/d/${dashboardId}?orgId=1&refresh=10s&kiosk`

  const pct = (used: number, total: number) => total > 0 ? Math.round((used / total) * 100) : 0

  const resources = summary ? [
    { label: t('dashboard.cpuUsage'), icon: <HddOutlined />, used: summary.cpu.allocated, total: summary.cpu.capacity, color: '#1677ff', unit: 'cores' },
    { label: t('dashboard.memUsage'), icon: <CloudServerOutlined />, used: summary.memory.allocated, total: summary.memory.capacity, color: '#722ed1', unit: 'GiB' },
    { label: t('dashboard.gpuUsage'), icon: <ThunderboltOutlined />, used: summary.gpu.allocated, total: summary.gpu.capacity, color: '#fa541c', unit: '' },
  ] : []

  const eventColumns = [
    { title: t('event.time'), dataIndex: 'lastTimestamp', key: 'time', width: 160, render: (ts: string) => ts ? new Date(ts).toLocaleString() : '-' },
    { title: t('event.reason'), dataIndex: 'reason', key: 'reason', width: 140, render: (r: string) => <Tag color="warning">{r}</Tag> },
    { title: t('event.message'), dataIndex: 'message', key: 'message', ellipsis: true },
    { title: 'Object', key: 'object', width: 160, render: (_: unknown, r: EventInfo) => <Text type="secondary" style={{ fontSize: 11 }}>{r.kind}/{r.name}</Text> },
  ]

  const overviewTab = (
    <div>
      {/* Resource Summary */}
      {loadingSummary ? (
        <div style={{ textAlign: 'center', padding: 32 }}><Spin /></div>
      ) : (
        <>
          {/* Node summary badges */}
          <Space style={{ marginBottom: 16 }}>
            <Tag color="blue">{summary?.nodes.total} {t('dashboard.nodes')}</Tag>
            <Tag color="green">{summary?.nodes.ready} Ready</Tag>
            {(summary?.nodes.notReady || 0) > 0 && <Tag color="red">{summary?.nodes.notReady} NotReady</Tag>}
            <Tag color="volcano">{summary?.nodes.gpu} GPU Nodes</Tag>
          </Space>

          <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
            {resources.map((r) => (
              <Col xs={24} md={8} key={r.label}>
                <Card size="small" bordered={false} style={{ borderRadius: 8, background: '#fafafa' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 10 }}>
                    <div style={{ color: r.color, fontSize: 18 }}>{r.icon}</div>
                    <Text strong style={{ fontSize: 13 }}>{r.label}</Text>
                  </div>
                  <Progress
                    percent={pct(r.used, r.total)}
                    strokeColor={r.color}
                    size="small"
                  />
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 4 }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t('dashboard.allocated')}: {r.used} {r.unit}</Text>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t('dashboard.total')}: {r.total} {r.unit}</Text>
                  </div>
                </Card>
              </Col>
            ))}
          </Row>
        </>
      )}

      {/* Grafana Cluster Dashboard */}
      <Card size="small" bordered={false} style={{ borderRadius: 8 }}>
        <iframe src={buildUrl('kube-ai-cluster-details')} style={iframeStyle} />
      </Card>
    </div>
  )

  const eventsTab = (
    <Card bordered={false} style={{ borderRadius: 10 }}>
      <div style={{ marginBottom: 12 }}>
        <Space>
          <WarningOutlined style={{ color: '#faad14' }} />
          <Title level={5} style={{ margin: 0 }}>{t('event.warnings')}</Title>
          <Badge count={events.length} style={{ background: '#faad14' }} />
        </Space>
      </div>
      <Table
        columns={eventColumns}
        dataSource={events}
        rowKey={(r, i) => `${r.namespace}/${r.name}/${i}`}
        pagination={{ pageSize: 15 }}
        size="small"
      />
    </Card>
  )

  const tabs = [
    { key: 'overview', label: t('dashboard.cluster'), children: overviewTab },
    { key: 'nodes', label: t('dashboard.nodes'), children: <Card bordered={false} style={{ borderRadius: 8 }}><iframe src={buildUrl('kube-ai-node-details')} style={iframeStyle} /></Card> },
    { key: 'gpu', label: t('dashboard.gpu'), children: <Card bordered={false} style={{ borderRadius: 8 }}><iframe src={buildUrl('kube-ai-gpu-details')} style={iframeStyle} /></Card> },
    { key: 'events', label: <Space><WarningOutlined />{t('dashboard.events')}</Space>, children: eventsTab },
  ]

  return <Tabs defaultActiveKey="overview" items={tabs} />
}
