import { useState, useEffect } from 'react'
import { Table, Tag, Card, Space, Button, Badge, Tooltip, Typography, Modal, Input, Spin, message, Checkbox } from 'antd'
import { ReloadOutlined, CheckCircleOutlined, CloseCircleOutlined, WarningOutlined, CodeOutlined, ThunderboltOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { get, post, getErrorMessage } from '@/api/client'

const { Text, Paragraph } = Typography

interface NodeInfo {
  name: string
  ip: string
  status: string
  roles: string
  version: string
  runtime: string
  os: string
  gpuCapacity: number
  gpuAllocatable: number
  gpuHealthy: boolean
  conditions: { type: string; status: string; reason: string; message: string }[]
}

interface ExecResult {
  nodeName: string
  stdout: string
  stderr: string
  error?: string
}

export default function Nodes() {
  const { t } = useTranslation()
  const [nodes, setNodes] = useState<NodeInfo[]>([])
  const [loading, setLoading] = useState(true)
  // Shell modal
  const [shellOpen, setShellOpen] = useState(false)
  const [shellNode, setShellNode] = useState('')
  const [shellCmd, setShellCmd] = useState('')
  const [shellResult, setShellResult] = useState<ExecResult | null>(null)
  const [shellLoading, setShellLoading] = useState(false)
  // Batch exec modal
  const [batchOpen, setBatchOpen] = useState(false)
  const [batchCmd, setBatchCmd] = useState('')
  const [batchNodes, setBatchNodes] = useState<string[]>([])
  const [batchResults, setBatchResults] = useState<ExecResult[]>([])
  const [batchLoading, setBatchLoading] = useState(false)

  const fetchNodes = () => {
    setLoading(true)
    get<NodeInfo[]>('/ops/nodes')
      .then((data: any) => setNodes(data || []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }

  const handleShellExec = async () => {
    if (!shellCmd.trim()) return
    setShellLoading(true)
    setShellResult(null)
    try {
      const result: any = await post('/ops/node-shell/exec', { nodeName: shellNode, command: shellCmd })
      setShellResult(result)
    } catch (err) {
      setShellResult({ nodeName: shellNode, stdout: '', stderr: '', error: getErrorMessage(err) })
    } finally {
      setShellLoading(false)
    }
  }

  const handleBatchExec = async () => {
    if (!batchCmd.trim() || batchNodes.length === 0) {
      message.warning('Select nodes and enter a command')
      return
    }
    setBatchLoading(true)
    setBatchResults([])
    try {
      const results: any = await post('/ops/node-shell/batch-exec', { nodeNames: batchNodes, command: batchCmd })
      setBatchResults(results || [])
    } catch (err) {
      message.error(getErrorMessage(err))
    } finally {
      setBatchLoading(false)
    }
  }

  const openShell = (nodeName: string) => {
    setShellNode(nodeName)
    setShellCmd('')
    setShellResult(null)
    setShellOpen(true)
  }

  useEffect(() => { fetchNodes() }, [])

  const columns = [
    {
      title: t('node.hostname'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: NodeInfo) => (
        <Space direction="vertical" size={0}>
          <Text strong style={{ fontSize: 13 }}>{name}</Text>
          <Text type="secondary" style={{ fontSize: 11 }}>{record.ip}</Text>
        </Space>
      ),
    },
    {
      title: t('common.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => (
        <Badge
          status={status === 'Ready' ? 'success' : 'error'}
          text={status === 'Ready' ? t('node.ready') : t('node.notReady')}
        />
      ),
    },
    {
      title: t('node.gpu'),
      key: 'gpu',
      width: 140,
      render: (_: unknown, record: NodeInfo) => {
        if (record.gpuCapacity === 0) return <Tag>CPU Only</Tag>
        return (
          <Space>
            <Tag color={record.gpuHealthy ? 'green' : 'red'}>
              {record.gpuAllocatable}/{record.gpuCapacity} GPU
            </Tag>
            {!record.gpuHealthy && (
              <Tooltip title={`${record.gpuCapacity - record.gpuAllocatable} GPU(s) unhealthy`}>
                <WarningOutlined style={{ color: '#ff4d4f' }} />
              </Tooltip>
            )}
          </Space>
        )
      },
    },
    {
      title: t('node.k8sVersion'),
      dataIndex: 'version',
      key: 'version',
      width: 140,
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: t('node.runtime'),
      dataIndex: 'runtime',
      key: 'runtime',
      width: 160,
      ellipsis: true,
    },
    {
      title: t('node.conditions'),
      key: 'conditions',
      width: 200,
      render: (_: unknown, record: NodeInfo) => {
        // Negative conditions: True = problem. Positive conditions (Ready, SufficientIP): True = normal.
        const negativeConditions = [
          'MemoryPressure', 'DiskPressure', 'PIDPressure', 'NodePIDPressure',
          'NetworkUnavailable', 'KernelDeadlock', 'ReadonlyFilesystem',
          'RuntimeOffline', 'DockerOffline', 'SystemdOffline', 'NTPProblem',
          'InodesPressure', 'InstanceExpired',
        ]
        const abnormal = record.conditions.filter(
          c => negativeConditions.includes(c.type) && c.status === 'True'
        )
        if (abnormal.length === 0) {
          return <Tag icon={<CheckCircleOutlined />} color="success">Healthy</Tag>
        }
        return (
          <Space size={4} wrap>
            {abnormal.map(c => (
              <Tooltip key={c.type} title={c.message}>
                <Tag icon={<CloseCircleOutlined />} color="error">{c.type}</Tag>
              </Tooltip>
            ))}
          </Space>
        )
      },
    },
  ]

  // Add actions column
  const allColumns = [
    ...columns,
    {
      title: t('common.actions'),
      key: 'actions',
      width: 100,
      render: (_: unknown, record: NodeInfo) => (
        <Button size="small" icon={<CodeOutlined />} onClick={() => openShell(record.name)}>
          Shell
        </Button>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={5} style={{ margin: 0 }}>{t('node.title')}</Typography.Title>
        <Space>
          <Button icon={<ThunderboltOutlined />} onClick={() => { setBatchNodes(nodes.map(n => n.name)); setBatchOpen(true) }}>
            Batch Exec
          </Button>
          <Button icon={<ReloadOutlined />} onClick={fetchNodes} loading={loading}>{t('common.refresh')}</Button>
        </Space>
      </div>
      <Card bordered={false} style={{ borderRadius: 10 }}>
        <Table
          columns={allColumns}
          dataSource={nodes}
          rowKey="name"
          loading={loading}
          pagination={false}
          size="middle"
        />
      </Card>

      {/* Single Node Shell Modal */}
      <Modal
        title={<Space><CodeOutlined /> Node Shell: {shellNode}</Space>}
        open={shellOpen}
        onCancel={() => setShellOpen(false)}
        footer={null}
        width={720}
      >
        <Space.Compact style={{ width: '100%', marginBottom: 12 }}>
          <Input
            placeholder="e.g. nvidia-smi, df -h, free -m, uptime"
            value={shellCmd}
            onChange={e => setShellCmd(e.target.value)}
            onPressEnter={handleShellExec}
            style={{ fontFamily: 'monospace' }}
          />
          <Button type="primary" onClick={handleShellExec} loading={shellLoading}>
            Execute
          </Button>
        </Space.Compact>
        {shellLoading && <Spin style={{ display: 'block', margin: '20px auto' }} />}
        {shellResult && (
          <div>
            {shellResult.error && (
              <Paragraph type="danger" style={{ fontFamily: 'monospace', fontSize: 12 }}>
                Error: {shellResult.error}
              </Paragraph>
            )}
            {shellResult.stdout && (
              <pre style={{
                background: '#1e1e1e', color: '#d4d4d4', padding: 12, borderRadius: 6,
                fontSize: 12, fontFamily: "'JetBrains Mono', 'Fira Code', Consolas, monospace",
                maxHeight: 400, overflow: 'auto', whiteSpace: 'pre-wrap',
              }}>
                {shellResult.stdout}
              </pre>
            )}
            {shellResult.stderr && (
              <pre style={{
                background: '#2d1b1b', color: '#f87171', padding: 12, borderRadius: 6,
                fontSize: 12, fontFamily: 'monospace', maxHeight: 200, overflow: 'auto',
              }}>
                {shellResult.stderr}
              </pre>
            )}
          </div>
        )}
      </Modal>

      {/* Batch Exec Modal */}
      <Modal
        title={<Space><ThunderboltOutlined /> Batch Execute on Nodes</Space>}
        open={batchOpen}
        onCancel={() => setBatchOpen(false)}
        footer={null}
        width={800}
      >
        <div style={{ marginBottom: 12 }}>
          <Text strong style={{ display: 'block', marginBottom: 8 }}>Target Nodes:</Text>
          <Checkbox.Group
            value={batchNodes}
            onChange={(vals) => setBatchNodes(vals as string[])}
            options={nodes.map(n => ({ label: `${n.name} (${n.ip})`, value: n.name }))}
            style={{ display: 'flex', flexDirection: 'column', gap: 4 }}
          />
        </div>
        <Space.Compact style={{ width: '100%', marginBottom: 12 }}>
          <Input
            placeholder="Command to run on all selected nodes"
            value={batchCmd}
            onChange={e => setBatchCmd(e.target.value)}
            onPressEnter={handleBatchExec}
            style={{ fontFamily: 'monospace' }}
          />
          <Button type="primary" onClick={handleBatchExec} loading={batchLoading}>
            Execute ({batchNodes.length} nodes)
          </Button>
        </Space.Compact>
        {batchLoading && <Spin style={{ display: 'block', margin: '20px auto' }} />}
        {batchResults.length > 0 && (
          <div style={{ maxHeight: 400, overflow: 'auto' }}>
            {batchResults.map((r, i) => (
              <Card key={i} size="small" style={{ marginBottom: 8, borderRadius: 6 }}
                title={<Space><Badge status={r.error ? 'error' : 'success'} /><Text style={{ fontSize: 12 }}>{r.nodeName}</Text></Space>}
              >
                {r.error && <Text type="danger" style={{ fontSize: 11 }}>Error: {r.error}</Text>}
                {r.stdout && (
                  <pre style={{ background: '#f5f5f5', padding: 8, borderRadius: 4, fontSize: 11, margin: 0, maxHeight: 120, overflow: 'auto' }}>
                    {r.stdout}
                  </pre>
                )}
                {r.stderr && (
                  <pre style={{ background: '#fff2f0', padding: 8, borderRadius: 4, fontSize: 11, margin: 0, color: '#cf1322' }}>
                    {r.stderr}
                  </pre>
                )}
              </Card>
            ))}
          </div>
        )}
      </Modal>
    </div>
  )
}
