import { useState, useEffect, useCallback } from 'react'
import { Tree, Button, Space, Modal, Form, Input, InputNumber, Select, Card, Row, Col, message, Popconfirm, Empty, Spin, Segmented, Tag, Progress, Typography } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, ReloadOutlined, ApartmentOutlined, FolderOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { fetchQuotaTrees, updateQuotaTree, type QuotaTree, type QuotaNode } from '@/api/quota'
import { get, getErrorMessage } from '@/api/client'
import type { DataNode } from 'antd/es/tree'

const { Text, Title } = Typography

// Resource colors for display


const RESOURCE_COLORS: Record<string, string> = {
  'cpu': '#1677ff',
  'memory': '#722ed1',
  'nvidia.com/gpu': '#fa541c',
  'aliyun.com/gpu': '#eb2f96',
  'aliyun.com/gpu-mem': '#13c2c2',
}

export default function QuotaTreePage() {
  const { t } = useTranslation()
  const [trees, setTrees] = useState<QuotaTree[]>([])
  const [selectedTree, setSelectedTree] = useState<QuotaTree | null>(null)
  const [selectedNode, setSelectedNode] = useState<QuotaNode | null>(null)
  const [selectedPath, setSelectedPath] = useState<string>('')
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [modalMode, setModalMode] = useState<'add' | 'edit'>('add')
  const [viewMode, setViewMode] = useState<string>(t('quota.formatted'))
  const [namespaceOptions, setNamespaceOptions] = useState<string[]>([])
  const [form] = Form.useForm()

  const loadTrees = useCallback(async () => {
    setLoading(true)
    try {
      const data = await fetchQuotaTrees()
      setTrees(data || [])
      if (data && data.length > 0) setSelectedTree(data[0])
    } catch { /* */ } finally { setLoading(false) }
  }, [])

  const loadNamespaces = useCallback(async () => {
    try {
      const data: any = await get('/k8s/namespace/list')
      const items = data?.items || data
      if (Array.isArray(items)) {
        setNamespaceOptions(items.map((ns: any) => ns.name || ns))
      }
    } catch { /* fallback */ }
  }, [])

  useEffect(() => { loadTrees(); loadNamespaces() }, [loadTrees, loadNamespaces])

  const buildTreeData = (node: QuotaNode, depth = 0): DataNode => ({
    key: node.name,
    title: (
      <Space size={4}>
        {depth === 0 ? <ApartmentOutlined style={{ color: '#1677ff' }} /> : <FolderOutlined style={{ color: '#8c8c8c' }} />}
        <span>{node.name.split('.').pop()}</span>
        {node.namespaces && node.namespaces.length > 0 && (
          <Tag color="blue" style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>
            {node.namespaces[0]}
          </Tag>
        )}
      </Space>
    ),
    children: (node.children || []).map(c => buildTreeData(c, depth + 1)),
  })

  const findNode = (root: QuotaNode, path: string): QuotaNode | null => {
    if (root.name === path) return root
    for (const child of root.children || []) {
      const found = findNode(child, path)
      if (found) return found
    }
    return null
  }

  const onSelect = (keys: React.Key[]) => {
    if (keys.length === 0 || !selectedTree?.spec?.root) return
    const path = keys[0] as string
    const node = findNode(selectedTree.spec.root, path)
    setSelectedNode(node)
    setSelectedPath(path)
  }

  const handleAdd = () => {
    setModalMode('add')
    form.resetFields()
    form.setFieldsValue({ resourceType: 'cpu', min: 0, max: 0 })
    setModalOpen(true)
  }

  const handleEdit = () => {
    if (!selectedNode) return
    setModalMode('edit')
    form.setFieldsValue({
      name: selectedNode.name.split('.').pop(),
      minCpu: selectedNode.min?.cpu || '',
      maxCpu: selectedNode.max?.cpu || '',
      minMemory: selectedNode.min?.memory || '',
      maxMemory: selectedNode.max?.memory || '',
      minGpu: selectedNode.min?.['nvidia.com/gpu'] || '',
      maxGpu: selectedNode.max?.['nvidia.com/gpu'] || '',
      namespaces: selectedNode.namespaces || [],
    })
    setModalOpen(true)
  }

  const handleDelete = async () => {
    if (!selectedNode || !selectedTree) return
    if (selectedPath === selectedTree.spec.root.name) {
      message.warning(t('quota.cannotDeleteRoot'))
      return
    }
    try {
      await updateQuotaTree({ action: 'delete', treeName: selectedTree.metadata.name, oldNodeName: selectedPath }, {})
      message.success(t('quota.nodeDeleted'))
      loadTrees()
      setSelectedNode(null)
    } catch (err) { message.error(getErrorMessage(err)) }
  }

  const buildResourceMaps = (values: any) => {
    const min: Record<string, string> = {}
    const max: Record<string, string> = {}
    if (values.minCpu) min['cpu'] = String(values.minCpu)
    if (values.maxCpu) max['cpu'] = String(values.maxCpu)
    if (values.minMemory) min['memory'] = String(values.minMemory)
    if (values.maxMemory) max['memory'] = String(values.maxMemory)
    if (values.minGpu) min['nvidia.com/gpu'] = String(values.minGpu)
    if (values.maxGpu) max['nvidia.com/gpu'] = String(values.maxGpu)
    return { min, max }
  }

  const handleModalOk = async () => {
    try {
      const values = await form.validateFields()
      if (!selectedTree) return
      const { min, max } = buildResourceMaps(values)
      if (modalMode === 'add') {
        const parentPath = selectedPath || selectedTree.spec.root.name
        const newName = `${parentPath}.${values.name}`
        const namespaces = values.namespaces || []
        await updateQuotaTree(
          { action: 'add', treeName: selectedTree.metadata.name, oldNodeName: parentPath },
          { name: newName, min, max, namespaces },
        )
        message.success(t('quota.nodeAdded'))
      } else {
        const namespaces = values.namespaces || []
        await updateQuotaTree(
          { action: 'update', treeName: selectedTree.metadata.name, oldNodeName: selectedPath },
          { name: selectedPath, min, max, namespaces },
        )
        message.success(t('quota.nodeUpdated'))
      }
      setModalOpen(false)
      loadTrees()
    } catch (err: any) {
      if (err?.errorFields) return
      message.error(getErrorMessage(err))
    }
  }

  const isRoot = selectedTree?.spec?.root?.name === selectedPath

  // Render formatted detail view for selected node
  const renderNodeDetail = () => {
    if (!selectedNode) {
      return <Empty description={t('quota.selectNode')} style={{ padding: 40 }} />
    }

    const allResources = new Set<string>()
    if (selectedNode.min) Object.keys(selectedNode.min).forEach(k => allResources.add(k))
    if (selectedNode.max) Object.keys(selectedNode.max).forEach(k => allResources.add(k))

    return (
      <div>
        <div style={{ marginBottom: 16 }}>
          <Title level={5} style={{ margin: 0 }}>{selectedNode.name.split('.').pop()}</Title>
          <Text type="secondary" style={{ fontSize: 12 }}>{selectedNode.name}</Text>
        </div>

        {/* Resource quotas as progress bars */}
        <div style={{ marginBottom: 20 }}>
          <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>{t('quota.resourceType')}</Text>
          {allResources.size === 0 ? (
            <Text type="secondary">{t('common.noData')}</Text>
          ) : (
            Array.from(allResources).map(resource => {
              const min = Number(selectedNode.min?.[resource] || 0)
              const max = Number(selectedNode.max?.[resource] || 0)
              const color = RESOURCE_COLORS[resource] || '#8c8c8c'
              return (
                <div key={resource} style={{ marginBottom: 12, padding: '8px 12px', background: '#fafafa', borderRadius: 6 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                    <Tag color={color} style={{ margin: 0 }}>{resource}</Tag>
                    <Text style={{ fontSize: 12 }}>
                      min: <Text strong>{min}</Text> / max: <Text strong>{max}</Text>
                    </Text>
                  </div>
                  <Progress
                    percent={max > 0 ? Math.round((min / max) * 100) : 0}
                    strokeColor={color}
                    size="small"
                    format={() => `${min}/${max}`}
                  />
                </div>
              )
            })
          )}
        </div>

        {/* Namespaces */}
        <div style={{ marginBottom: 16 }}>
          <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>{t('quota.namespaces')}</Text>
          {(selectedNode.namespaces || []).length > 0 ? (
            <Space wrap>
              {selectedNode.namespaces!.map(ns => <Tag key={ns} color="blue">{ns}</Tag>)}
            </Space>
          ) : (
            <Text type="secondary">—</Text>
          )}
        </div>

        {/* Children */}
        {(selectedNode.children || []).length > 0 && (
          <div>
            <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>{t('common.childNodes')}</Text>
            <Space wrap>
              {selectedNode.children!.map(c => (
                <Tag key={c.name}>{c.name.split('.').pop()}</Tag>
              ))}
            </Space>
          </div>
        )}
      </div>
    )
  }

  // Render raw JSON view
  const renderRawView = () => {
    if (!selectedTree) return <Empty description={t('quota.selectTree')} />
    const jsonStr = JSON.stringify(selectedTree, null, 2)
    return (
      <pre style={{
        background: '#1e1e1e',
        color: '#d4d4d4',
        padding: 16,
        borderRadius: 8,
        fontSize: 12,
        fontFamily: "'JetBrains Mono', 'Fira Code', 'SF Mono', Consolas, monospace",
        overflow: 'auto',
        maxHeight: 'calc(100vh - 240px)',
        lineHeight: 1.6,
        margin: 0,
      }}>
        {jsonStr}
      </pre>
    )
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Space>
          <Select
            style={{ width: 260 }}
            placeholder={t('quota.selectTree')}
            value={selectedTree?.metadata?.name}
            onChange={(v) => {
              const tree = trees.find(tr => tr.metadata.name === v)
              setSelectedTree(tree || null)
              setSelectedNode(null)
              setSelectedPath('')
            }}
            options={trees.map(tr => ({ label: tr.metadata.name, value: tr.metadata.name }))}
          />
          <Button icon={<ReloadOutlined />} onClick={loadTrees} loading={loading}>{t('common.refresh')}</Button>
        </Space>
        <Segmented
          options={[t('quota.formatted'), t('quota.rawYaml')]}
          value={viewMode}
          onChange={(v) => setViewMode(v as string)}
        />
      </div>

      {viewMode === t('quota.rawYaml') ? (
        <Card bordered={false} style={{ borderRadius: 10 }}>
          {renderRawView()}
        </Card>
      ) : (
        <Row gutter={16}>
          <Col span={10}>
            <Card
              bordered={false}
              style={{ borderRadius: 10, minHeight: 400 }}
              title={
                <Space>
                  <ApartmentOutlined />
                  <span>{t('quota.title')}</span>
                </Space>
              }
              extra={
                <Space size={4}>
                  <Button size="small" type="primary" ghost icon={<PlusOutlined />} onClick={handleAdd} disabled={!selectedTree}>
                    {t('quota.addNode')}
                  </Button>
                  <Button size="small" icon={<EditOutlined />} onClick={handleEdit} disabled={!selectedNode} />
                  <Popconfirm title={t('common.confirmDelete')} onConfirm={handleDelete} disabled={!selectedNode || isRoot}>
                    <Button size="small" danger icon={<DeleteOutlined />} disabled={!selectedNode || isRoot} />
                  </Popconfirm>
                </Space>
              }
            >
              {loading ? (
                <div style={{ textAlign: 'center', padding: 32 }}><Spin /></div>
              ) : selectedTree?.spec?.root ? (
                <Tree
                  treeData={[buildTreeData(selectedTree.spec.root)]}
                  defaultExpandAll
                  onSelect={onSelect}
                  selectedKeys={selectedPath ? [selectedPath] : []}
                  style={{ fontSize: 13 }}
                />
              ) : (
                <Empty description={t('common.noData')} />
              )}
            </Card>
          </Col>
          <Col span={14}>
            <Card bordered={false} style={{ borderRadius: 10, minHeight: 400 }} title={t('quota.nodeDetail')}>
              {renderNodeDetail()}
            </Card>
          </Col>
        </Row>
      )}

      <Modal
        title={modalMode === 'add' ? t('quota.addNode') : t('quota.editNode')}
        open={modalOpen}
        onOk={handleModalOk}
        onCancel={() => setModalOpen(false)}
        okText={t('common.confirm')}
        cancelText={t('common.cancel')}
        width={600}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
            <Input disabled={modalMode === 'edit'} placeholder="node-name" />
          </Form.Item>

          {/* CPU */}
          <div style={{ background: '#f9f9fb', borderRadius: 10, padding: '12px 16px', marginBottom: 12 }}>
            <Text strong style={{ fontSize: 13, color: '#1677ff' }}>{t('quota.cpu')}</Text>
            <Row gutter={16} style={{ marginTop: 8 }}>
              <Col span={12}>
                <Form.Item name="minCpu" label={t('quota.min')} style={{ marginBottom: 0 }}>
                  <InputNumber style={{ width: '100%' }} min={0} placeholder="e.g. 4" addonAfter="cores" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="maxCpu" label={t('quota.max')} style={{ marginBottom: 0 }}>
                  <InputNumber style={{ width: '100%' }} min={0} placeholder="e.g. 16" addonAfter="cores" />
                </Form.Item>
              </Col>
            </Row>
          </div>

          {/* Memory */}
          <div style={{ background: '#f9f9fb', borderRadius: 10, padding: '12px 16px', marginBottom: 12 }}>
            <Text strong style={{ fontSize: 13, color: '#722ed1' }}>{t('quota.memory')}</Text>
            <Row gutter={16} style={{ marginTop: 8 }}>
              <Col span={12}>
                <Form.Item name="minMemory" label={t('quota.min')} style={{ marginBottom: 0 }}>
                  <Input placeholder="e.g. 16Gi" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="maxMemory" label={t('quota.max')} style={{ marginBottom: 0 }}>
                  <Input placeholder="e.g. 64Gi" />
                </Form.Item>
              </Col>
            </Row>
          </div>

          {/* GPU */}
          <div style={{ background: '#f9f9fb', borderRadius: 10, padding: '12px 16px', marginBottom: 12 }}>
            <Text strong style={{ fontSize: 13, color: '#fa541c' }}>{t('quota.gpu')} (nvidia.com/gpu)</Text>
            <Row gutter={16} style={{ marginTop: 8 }}>
              <Col span={12}>
                <Form.Item name="minGpu" label={t('quota.min')} style={{ marginBottom: 0 }}>
                  <InputNumber style={{ width: '100%' }} min={0} placeholder="e.g. 0" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="maxGpu" label={t('quota.max')} style={{ marginBottom: 0 }}>
                  <InputNumber style={{ width: '100%' }} min={0} placeholder="e.g. 8" />
                </Form.Item>
              </Col>
            </Row>
          </div>

          {/* Namespaces binding */}
          <Form.Item name="namespaces" label={t('quota.namespaces')} tooltip={modalMode === 'add' ? t('quota.nsBindTip') : undefined}>
            <Select
              mode="tags"
              placeholder={t('quota.nsPlaceholder')}
              options={namespaceOptions.map(ns => ({ label: ns, value: ns }))}
              tokenSeparators={[',']}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
