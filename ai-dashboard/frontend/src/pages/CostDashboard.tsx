import { useEffect, useState } from 'react'
import { Card, Table, Tag, Space, Typography, Select, Progress, Row, Col, Statistic, Badge, Button } from 'antd'
import { DollarOutlined, ReloadOutlined, ClusterOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { get } from '@/api/client'

const { Title, Text } = Typography

interface QuotaUsageInfo {
  name: string
  min: Record<string, string>
  max: Record<string, string>
  used: Record<string, string>
  namespaces: string[]
  children?: QuotaUsageInfo[]
}

interface GPUUsageRecord {
  user: string
  namespace: string
  gpuHours: number
  jobCount: number
}

interface CostSummary {
  quotaTree: QuotaUsageInfo | null
  gpuUsage: GPUUsageRecord[]
  totalGpuHours: number
  timeRange: string
}

export default function CostDashboard() {
  const { t } = useTranslation()
  const [data, setData] = useState<CostSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [timeRange, setTimeRange] = useState('7d')

  const fetchData = (range: string) => {
    setLoading(true)
    get<CostSummary>(`/ops/gpu-usage`, { params: { range } })
      .then(setData)
      .catch(() => setData(null))
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchData(timeRange) }, [timeRange])

  const usageColumns = [
    {
      title: t('cost.user'),
      dataIndex: 'user',
      key: 'user',
      render: (v: string) => <Text strong>{v}</Text>,
    },
    {
      title: t('cost.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: t('cost.gpuHours'),
      dataIndex: 'gpuHours',
      key: 'gpuHours',
      sorter: (a: GPUUsageRecord, b: GPUUsageRecord) => a.gpuHours - b.gpuHours,
      render: (v: number) => <Text strong style={{ color: '#0071e3' }}>{v.toFixed(1)}</Text>,
    },
    {
      title: t('cost.jobCount'),
      dataIndex: 'jobCount',
      key: 'jobCount',
      width: 80,
      render: (v: number) => <Badge count={v} showZero style={{ backgroundColor: '#52c41a' }} />,
    },
  ]

  // Parse GPU value from resource string (e.g. "8" or "0")
  const parseResourceValue = (val: string | undefined): number => {
    if (!val) return 0
    const n = parseInt(val, 10)
    return isNaN(n) ? 0 : n
  }

  // Render quota tree as nested cards
  const renderQuotaNode = (node: QuotaUsageInfo, depth = 0): React.ReactNode => {
    const gpuMin = parseResourceValue(node.min?.['nvidia.com/gpu'])
    const gpuMax = parseResourceValue(node.max?.['nvidia.com/gpu'])
    const gpuUsed = parseResourceValue(node.used?.['nvidia.com/gpu'])
    const pct = gpuMax > 0 ? Math.round((gpuUsed / gpuMax) * 100) : 0

    return (
      <div key={node.name} style={{ marginLeft: depth * 16, marginBottom: 8 }}>
        <div style={{ background: depth === 0 ? '#f0f5ff' : '#f9f9fb', borderRadius: 10, padding: '12px 16px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
            <Space>
              <ClusterOutlined style={{ color: '#0071e3' }} />
              <Text strong>{node.name}</Text>
              {node.namespaces?.length > 0 && (
                <Text type="secondary" style={{ fontSize: 11 }}>({node.namespaces.join(', ')})</Text>
              )}
            </Space>
            <Space size={16}>
              <span style={{ fontSize: 11 }}>{t('cost.min')}: <Tag>{gpuMin} GPU</Tag></span>
              <span style={{ fontSize: 11 }}>{t('cost.max')}: <Tag>{gpuMax} GPU</Tag></span>
              <span style={{ fontSize: 11 }}>{t('cost.used')}: <Tag color={pct > 80 ? 'red' : pct > 50 ? 'orange' : 'green'}>{gpuUsed} GPU</Tag></span>
            </Space>
          </div>
          <Progress percent={pct} strokeColor={pct > 80 ? '#ff4d4f' : pct > 50 ? '#faad14' : '#52c41a'} size="small" />
        </div>
        {node.children?.map((child) => renderQuotaNode(child, depth + 1))}
      </div>
    )
  }

  return (
    <div style={{ padding: '0 4px' }}>
      <div style={{ marginBottom: 20, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <div>
          <Title level={4} style={{ marginBottom: 4 }}>{t('cost.title')}</Title>
          <Text type="secondary">{t('cost.desc')}</Text>
        </div>
        <Space>
          <Select value={timeRange} onChange={setTimeRange} style={{ width: 120 }} options={[
            { label: '24h', value: '24h' },
            { label: '7 days', value: '7d' },
            { label: '30 days', value: '30d' },
          ]} />
          <Button icon={<ReloadOutlined />} onClick={() => fetchData(timeRange)}>{t('common.refresh')}</Button>
        </Space>
      </div>

      {/* Summary Stats */}
      <Row gutter={16} style={{ marginBottom: 20 }}>
        <Col span={8}>
          <Card style={{ borderRadius: 14, border: 'none', boxShadow: '0 1px 3px rgba(0,0,0,0.04)' }}>
            <Statistic
              title={t('cost.totalGpuHours')}
              value={data?.totalGpuHours || 0}
              precision={1}
              suffix="h"
              prefix={<DollarOutlined style={{ color: '#0071e3' }} />}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card style={{ borderRadius: 14, border: 'none', boxShadow: '0 1px 3px rgba(0,0,0,0.04)' }}>
            <Statistic
              title={t('cost.gpuUsage') + ' (' + t('cost.user') + ')'}
              value={data?.gpuUsage?.length || 0}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card style={{ borderRadius: 14, border: 'none', boxShadow: '0 1px 3px rgba(0,0,0,0.04)' }}>
            <Statistic
              title={t('cost.timeRange')}
              value={timeRange}
              valueStyle={{ fontSize: 24 }}
            />
          </Card>
        </Col>
      </Row>

      {/* Quota Tree Visualization */}
      {data?.quotaTree && (
        <Card
          title={<Space><ClusterOutlined />{t('cost.quotaTree')}</Space>}
          style={{ borderRadius: 14, border: 'none', boxShadow: '0 1px 3px rgba(0,0,0,0.04)', marginBottom: 20 }}
          styles={{ body: { padding: 16 } }}
          loading={loading}
        >
          {renderQuotaNode(data.quotaTree)}
        </Card>
      )}

      {/* GPU Usage Table */}
      <Card
        title={<Space><DollarOutlined />{t('cost.gpuUsage')}</Space>}
        style={{ borderRadius: 14, border: 'none', boxShadow: '0 1px 3px rgba(0,0,0,0.04)' }}
        styles={{ body: { padding: 0 } }}
        loading={loading}
      >
        <Table
          columns={usageColumns}
          dataSource={data?.gpuUsage || []}
          rowKey={(r) => `${r.user}/${r.namespace}`}
          pagination={{ pageSize: 20, showSizeChanger: false }}
          locale={{ emptyText: t('common.noData') }}
        />
      </Card>
    </div>
  )
}
