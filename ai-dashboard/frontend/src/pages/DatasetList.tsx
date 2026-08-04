import { useState, useEffect, useCallback } from 'react'
import { Table, Button, Space, Tag, message, Popconfirm, Card, Empty } from 'antd'
import { DeleteOutlined, ReloadOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { fetchDatasets, deleteDataset } from '@/api/dataset'
import { getErrorMessage } from '@/api/client'

export default function DatasetList() {
  const { t } = useTranslation()
  const [datasets, setDatasets] = useState<any[]>([])
  const [loading, setLoading] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await fetchDatasets()
      setDatasets(data?.items || [])
    } catch { /* */ } finally { setLoading(false) }
  }, [])

  useEffect(() => { load() }, [load])

  const handleDelete = async (name: string, namespace: string) => {
    try { await deleteDataset(name, namespace); message.success(t('dataset.deleted')); load() }
    catch (err) { message.error(getErrorMessage(err)) }
  }

  const columns = [
    { title: t('common.name'), key: 'name', render: (_: any, r: any) => r.metadata?.name },
    { title: t('common.namespace'), key: 'namespace', render: (_: any, r: any) => r.metadata?.namespace },
    {
      title: t('common.status'), key: 'status', width: 100,
      render: (_: any, r: any) => {
        const phase = r.status?.phase || 'Unknown'
        const color = phase === 'Bound' || phase === 'Ready' ? 'green' : phase === 'NotReady' ? 'red' : 'default'
        return <Tag color={color}>{phase}</Tag>
      },
    },
    {
      title: t('dataset.runtime'), key: 'runtime',
      render: (_: any, r: any) => {
        const runtimes = r.spec?.runtimes || []
        return runtimes.length ? runtimes.map((rt: any) => <Tag key={rt.type} color="purple">{rt.type}</Tag>) : '—'
      },
    },
    {
      title: t('common.actions'), key: 'actions', width: 100, align: 'center' as const,
      render: (_: any, r: any) => (
        <Popconfirm title={t('common.confirmDelete')} onConfirm={() => handleDelete(r.metadata.name, r.metadata.namespace)}>
          <Button size="small" danger icon={<DeleteOutlined />}>{t('common.delete')}</Button>
        </Popconfirm>
      ),
    },
  ]

  return (
    <Card size="small" title={t('dataset.title')} extra={
      <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>{t('common.refresh')}</Button>
    }>
      <Table
        dataSource={datasets}
        columns={columns}
        rowKey={(r: any) => `${r.metadata?.namespace}/${r.metadata?.name}`}
        loading={loading}
        size="small"
        pagination={{ pageSize: 20, showSizeChanger: true }}
        locale={{ emptyText: <Empty description={t('common.noData')} /> }}
      />
    </Card>
  )
}
