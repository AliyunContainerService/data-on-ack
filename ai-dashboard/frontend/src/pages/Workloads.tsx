import { useState, useEffect } from 'react'
import { Table, Tag, Card, Space, Button, Badge, Select, Typography } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { get } from '@/api/client'

const { Text } = Typography

interface WorkloadInfo {
  name: string
  namespace: string
  kind: string
  status: string
  owner: string
  gpu: number
  created: string
}

const kindColors: Record<string, string> = {
  Notebook: 'blue',
  PyTorchJob: 'purple',
  TFJob: 'orange',
  MPIJob: 'cyan',
}

const statusBadge: Record<string, 'success' | 'processing' | 'error' | 'warning' | 'default'> = {
  Running: 'processing',
  Succeeded: 'success',
  Failed: 'error',
  Pending: 'warning',
  Stopped: 'default',
}

export default function Workloads() {
  const { t } = useTranslation()
  const [workloads, setWorkloads] = useState<WorkloadInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [kindFilter, setKindFilter] = useState<string>('')

  const fetchWorkloads = () => {
    setLoading(true)
    get<WorkloadInfo[]>('/ops/workloads')
      .then((data: any) => setWorkloads(data || []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchWorkloads() }, [])

  const filtered = kindFilter
    ? workloads.filter(w => w.kind === kindFilter)
    : workloads

  const columns = [
    {
      title: t('common.name'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: WorkloadInfo) => (
        <Space direction="vertical" size={0}>
          <Text strong>{name}</Text>
          <Text type="secondary" style={{ fontSize: 11 }}>{record.namespace}</Text>
        </Space>
      ),
    },
    {
      title: t('common.type'),
      dataIndex: 'kind',
      key: 'kind',
      width: 120,
      render: (kind: string) => <Tag color={kindColors[kind] || 'default'}>{kind}</Tag>,
    },
    {
      title: t('common.status'),
      dataIndex: 'status',
      key: 'status',
      width: 110,
      render: (status: string) => (
        <Badge status={statusBadge[status] || 'default'} text={status} />
      ),
    },
    {
      title: t('workload.owner'),
      dataIndex: 'owner',
      key: 'owner',
      width: 160,
      ellipsis: true,
      render: (owner: string) => owner || '-',
    },
    {
      title: t('workload.gpuCount'),
      dataIndex: 'gpu',
      key: 'gpu',
      width: 80,
      render: (gpu: number) => gpu > 0 ? <Tag color="volcano">{gpu} GPU</Tag> : <Tag>CPU</Tag>,
    },
    {
      title: t('workload.created'),
      dataIndex: 'created',
      key: 'created',
      width: 170,
      render: (ts: string) => ts ? new Date(ts).toLocaleString() : '-',
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Space>
          <Typography.Title level={5} style={{ margin: 0 }}>{t('workload.title')}</Typography.Title>
          <Select
            allowClear
            placeholder="Filter by type"
            style={{ width: 150 }}
            value={kindFilter || undefined}
            onChange={(v) => setKindFilter(v || '')}
            options={[
              { label: 'Notebook', value: 'Notebook' },
              { label: 'PyTorchJob', value: 'PyTorchJob' },
              { label: 'TFJob', value: 'TFJob' },
              { label: 'MPIJob', value: 'MPIJob' },
            ]}
          />
        </Space>
        <Button icon={<ReloadOutlined />} onClick={fetchWorkloads} loading={loading}>{t('common.refresh')}</Button>
      </div>
      <Card bordered={false} style={{ borderRadius: 10 }}>
        <Table
          columns={columns}
          dataSource={filtered}
          rowKey={(r) => `${r.namespace}/${r.name}`}
          loading={loading}
          pagination={{ pageSize: 20 }}
          size="middle"
        />
      </Card>
    </div>
  )
}
