import { useEffect, useState } from 'react'
import {
  Card, Table, Tag, Space, Typography, Button, Drawer, Tabs, Badge,
  Empty, Descriptions, Modal, Form, Input,
  message,
} from 'antd'
import {
  AppstoreOutlined, ReloadOutlined, PlusOutlined, BranchesOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { get, post, getErrorMessage } from '@/api/client'

const { Title, Text } = Typography

interface ModelInfo {
  name: string
  namespace: string
  framework: string
  path: string
  source: string
  size: string
  versionCount: number
  latestVersion: string
  registerTime: string
}

interface ModelVersion {
  version: string
  trainedFrom: string
  datasetRef: string
  createdAt: string
  metrics: Record<string, number> | null
  status: string
  servingName: string
}

interface LineageNode {
  id: string
  type: string
  name: string
}

interface LineageEdge {
  from: string
  to: string
}

interface ModelLineage {
  nodes: LineageNode[]
  edges: LineageEdge[]
}

export default function ModelHub() {
  const { t } = useTranslation()
  const [models, setModels] = useState<ModelInfo[]>([])
  const [loading, setLoading] = useState(true)

  // Detail drawer
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailModel, setDetailModel] = useState<ModelInfo | null>(null)
  const [detailTab, setDetailTab] = useState('versions')
  const [versions, setVersions] = useState<ModelVersion[]>([])
  const [lineage, setLineage] = useState<ModelLineage | null>(null)
  const [versionsLoading, setVersionsLoading] = useState(false)

  // Create version modal
  const [createOpen, setCreateOpen] = useState(false)
  const [form] = Form.useForm()

  const fetchModels = () => {
    setLoading(true)
    get<ModelInfo[]>('/ops/models')
      .then((data) => setModels(data || []))
      .catch(() => setModels([]))
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchModels() }, [])

  const handleViewDetail = async (model: ModelInfo) => {
    setDetailModel(model)
    setDetailOpen(true)
    setDetailTab('versions')
    setVersionsLoading(true)
    try {
      const [vers, lin] = await Promise.all([
        get<ModelVersion[]>(`/ops/models/${model.name}/versions`, { params: { namespace: model.namespace } }),
        get<ModelLineage>(`/ops/models/${model.name}/lineage`, { params: { namespace: model.namespace } }),
      ])
      setVersions(vers || [])
      setLineage(lin || null)
    } catch {
      setVersions([])
      setLineage(null)
    } finally {
      setVersionsLoading(false)
    }
  }

  const handleCreateVersion = async () => {
    if (!detailModel) return
    try {
      const values = await form.validateFields()
      await post(`/ops/models/${detailModel.name}/versions?namespace=${detailModel.namespace}`, values)
      message.success(t('common.success'))
      setCreateOpen(false)
      form.resetFields()
      // Refresh versions
      const vers = await get<ModelVersion[]>(`/ops/models/${detailModel.name}/versions`, { params: { namespace: detailModel.namespace } })
      setVersions(vers || [])
    } catch (err: any) {
      if (err?.errorFields) return
      message.error(getErrorMessage(err))
    }
  }

  const columns = [
    {
      title: t('common.name'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: ModelInfo) => (
        <Button type="link" style={{ padding: 0, fontWeight: 500 }} onClick={() => handleViewDetail(record)}>
          {name}
        </Button>
      ),
    },
    {
      title: t('common.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: 'Framework',
      dataIndex: 'framework',
      key: 'framework',
      render: (v: string) => <Tag color="blue">{v}</Tag>,
    },
    {
      title: 'Size',
      dataIndex: 'size',
      key: 'size',
    },
    {
      title: t('model.versions'),
      dataIndex: 'versionCount',
      key: 'versionCount',
      render: (v: number) => <Badge count={v} showZero style={{ backgroundColor: '#0071e3' }} />,
    },
    {
      title: 'Latest',
      dataIndex: 'latestVersion',
      key: 'latestVersion',
      render: (v: string) => v ? <Tag color="green">{v}</Tag> : '-',
    },
    {
      title: 'Registered',
      dataIndex: 'registerTime',
      key: 'registerTime',
      render: (v: string) => v ? new Date(v).toLocaleDateString() : '-',
    },
  ]

  // Lineage DAG visualization using flexbox
  const renderLineage = () => {
    if (!lineage || lineage.nodes.length === 0) {
      return <Empty description="No lineage data" />
    }

    const typeOrder = ['dataset', 'training', 'model', 'serving']
    const grouped = typeOrder.map(type => lineage.nodes.filter(n => n.type === type))

    const typeColors: Record<string, string> = {
      dataset: '#52c41a',
      training: '#faad14',
      model: '#0071e3',
      serving: '#722ed1',
    }

    const typeLabels: Record<string, string> = {
      dataset: 'Dataset',
      training: 'Training',
      model: 'Model',
      serving: 'Serving',
    }

    return (
      <div>
        <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>{t('model.dagDesc')}</Text>
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8, overflowX: 'auto', padding: '16px 0' }}>
          {grouped.map((nodes, groupIdx) => (
            <div key={groupIdx} style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', minWidth: 120 }}>
              {/* Type label */}
              <Text type="secondary" style={{ fontSize: 10, marginBottom: 8 }}>{typeLabels[typeOrder[groupIdx]]}</Text>
              {/* Nodes */}
              {nodes.map((node) => (
                <div
                  key={node.id}
                  style={{
                    border: `2px solid ${typeColors[node.type]}`,
                    borderRadius: 10,
                    padding: '8px 14px',
                    marginBottom: 8,
                    background: `${typeColors[node.type]}10`,
                    textAlign: 'center',
                    minWidth: 100,
                  }}
                >
                  <Text strong style={{ fontSize: 11, display: 'block' }}>{node.name}</Text>
                  <Tag color={typeColors[node.type]} style={{ fontSize: 9, marginTop: 4 }}>{node.type}</Tag>
                </div>
              ))}
              {/* Arrow to next group */}
              {groupIdx < grouped.length - 1 && nodes.length > 0 && (
                <div style={{ position: 'absolute', right: -16, top: '50%' }}>
                  <Text type="secondary">&rarr;</Text>
                </div>
              )}
            </div>
          ))}
        </div>
        {/* Edges as text */}
        <div style={{ marginTop: 12, padding: '8px 12px', background: '#f9f9fb', borderRadius: 8 }}>
          <Text type="secondary" style={{ fontSize: 11 }}>Connections:</Text>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 4 }}>
            {lineage.edges.map((edge, idx) => (
              <Tag key={idx} style={{ fontSize: 10 }}>
                {edge.from.split('-').slice(1).join('-')} &rarr; {edge.to.split('-').slice(1).join('-')}
              </Tag>
            ))}
          </div>
        </div>
      </div>
    )
  }

  return (
    <div style={{ padding: '0 4px' }}>
      <div style={{ marginBottom: 20 }}>
        <Title level={4} style={{ marginBottom: 4 }}>{t('model.title')}</Title>
        <Text type="secondary">{t('model.desc')}</Text>
      </div>

      <Card
        title={<Space><AppstoreOutlined />{t('model.title')}<Badge count={models.length} style={{ backgroundColor: '#f0f0f0', color: '#666' }} /></Space>}
        extra={<Button icon={<ReloadOutlined />} onClick={fetchModels}>{t('common.refresh')}</Button>}
        style={{ borderRadius: 14, border: 'none', boxShadow: '0 1px 3px rgba(0,0,0,0.04)' }}
        styles={{ body: { padding: 0 } }}
        loading={loading}
      >
        <Table
          columns={columns}
          dataSource={models}
          rowKey={(r) => `${r.namespace}/${r.name}`}
          pagination={{ pageSize: 15, showSizeChanger: false }}
          locale={{ emptyText: <Empty description={t('common.noData')} /> }}
        />
      </Card>

      {/* Model Detail Drawer */}
      <Drawer
        title={<Space><AppstoreOutlined />{detailModel?.name} <Tag color="blue">{detailModel?.framework}</Tag></Space>}
        open={detailOpen}
        onClose={() => { setDetailOpen(false); setDetailModel(null) }}
        width={720}
        styles={{ body: { padding: 0 } }}
      >
        {detailModel && (
          <>
            <div style={{ padding: '16px 20px', background: '#f9f9fb', borderBottom: '1px solid #f0f0f0' }}>
              <Descriptions column={3} size="small">
                <Descriptions.Item label="Framework">{detailModel.framework}</Descriptions.Item>
                <Descriptions.Item label="Size">{detailModel.size}</Descriptions.Item>
                <Descriptions.Item label="Source">{detailModel.source}</Descriptions.Item>
                <Descriptions.Item label="Path" span={3}><Text code style={{ fontSize: 11 }}>{detailModel.path}</Text></Descriptions.Item>
              </Descriptions>
            </div>
            <Tabs activeKey={detailTab} onChange={setDetailTab} style={{ padding: '0 16px' }} items={[
              {
                key: 'versions',
                label: `${t('model.versions')} (${versions.length})`,
                children: (
                  <div style={{ padding: '0 0 16px' }}>
                    <div style={{ marginBottom: 12 }}>
                      <Button size="small" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
                        {t('model.createVersion')}
                      </Button>
                    </div>
                    {versionsLoading ? (
                      <div style={{ padding: 32, textAlign: 'center' }}>Loading...</div>
                    ) : versions.length === 0 ? (
                      <Empty description="No versions" />
                    ) : (
                      <table style={{ width: '100%', fontSize: 12, borderCollapse: 'collapse' }}>
                        <thead>
                          <tr style={{ background: '#f9f9fb', textAlign: 'left' }}>
                            <th style={{ padding: '8px 12px' }}>{t('model.version')}</th>
                            <th style={{ padding: '8px 12px' }}>{t('model.trainedFrom')}</th>
                            <th style={{ padding: '8px 12px' }}>{t('model.datasetRef')}</th>
                            <th style={{ padding: '8px 12px' }}>{t('model.status')}</th>
                            <th style={{ padding: '8px 12px' }}>{t('model.metrics')}</th>
                            <th style={{ padding: '8px 12px' }}>Created</th>
                          </tr>
                        </thead>
                        <tbody>
                          {versions.map((v, idx) => (
                            <tr key={idx} style={{ borderBottom: '1px solid #f0f0f0' }}>
                              <td style={{ padding: '8px 12px' }}><Tag color="blue">{v.version}</Tag></td>
                              <td style={{ padding: '8px 12px', fontFamily: 'monospace', fontSize: 11 }}>{v.trainedFrom || '-'}</td>
                              <td style={{ padding: '8px 12px' }}>{v.datasetRef || '-'}</td>
                              <td style={{ padding: '8px 12px' }}>
                                <Badge status={v.status === 'ready' ? 'success' : v.status === 'training' ? 'processing' : 'error'} text={v.status} />
                              </td>
                              <td style={{ padding: '8px 12px' }}>
                                {v.metrics ? Object.entries(v.metrics).map(([k, val]) => (
                                  <Tag key={k} style={{ fontSize: 10 }}>{k}: {val.toFixed(4)}</Tag>
                                )) : '-'}
                              </td>
                              <td style={{ padding: '8px 12px', fontSize: 11 }}>
                                {v.createdAt ? new Date(v.createdAt).toLocaleDateString() : '-'}
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    )}
                  </div>
                ),
              },
              {
                key: 'lineage',
                label: <Space><BranchesOutlined />{t('model.lineage')}</Space>,
                children: (
                  <div style={{ padding: '12px 0 16px' }}>
                    {renderLineage()}
                  </div>
                ),
              },
            ]} />
          </>
        )}
      </Drawer>

      {/* Create Version Modal */}
      <Modal
        title={t('model.createVersion')}
        open={createOpen}
        onOk={handleCreateVersion}
        onCancel={() => { setCreateOpen(false); form.resetFields() }}
        okText={t('common.confirm')}
        cancelText={t('common.cancel')}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="version" label={t('model.version')} rules={[{ required: true }]}>
            <Input placeholder="v1.0" />
          </Form.Item>
          <Form.Item name="trainedFrom" label={t('model.trainedFrom')}>
            <Input placeholder="training-job-name" />
          </Form.Item>
          <Form.Item name="datasetRef" label={t('model.datasetRef')}>
            <Input placeholder="dataset-name" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
