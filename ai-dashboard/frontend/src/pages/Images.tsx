import { useState, useEffect } from 'react'
import { Card, Tag, Space, Button, Typography, Input, Collapse, Table, Badge, Popconfirm, message } from 'antd'
import { ReloadOutlined, SearchOutlined, CloudServerOutlined, DeleteOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { get, post } from '@/api/client'

const { Text } = Typography

interface NodeImage {
  image: string
  sizeMB: number
  tags: string[]
}

interface NodeImageInfo {
  node: string
  ip: string
  gpu: number
  images: NodeImage[]
}

export default function Images() {
  const { t } = useTranslation()
  const [data, setData] = useState<NodeImageInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')

  const fetchData = () => {
    setLoading(true)
    get<NodeImageInfo[]>('/ops/images')
      .then((d: any) => setData(d || []))
      .catch(() => setData([]))
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchData() }, [])

  // Filter images by search keyword
  const filteredData = data.map(node => ({
    ...node,
    images: (node.images || []).filter(img =>
      img.image.toLowerCase().includes(search.toLowerCase())
    ),
  }))

  const handleDeleteImage = async (nodeName: string, image: string) => {
    try {
      await post('/ops/images/delete', { nodeName, image })
      message.success('Image deleted')
      fetchData()
    } catch (err) {
      message.error(err instanceof Error ? err.message : 'Delete failed')
    }
  }

  const getImageColumns = (nodeName: string) => [
    {
      title: 'Image',
      dataIndex: 'image',
      key: 'image',
      render: (img: string) => {
        const parts = img.split('/')
        const short = parts.length > 2 ? parts.slice(-2).join('/') : img
        const shortDisplay = short.split('@')[0] || short
        return (
          <Text style={{ fontSize: 12, fontFamily: 'monospace' }} copyable={{ text: img }} ellipsis>
            {shortDisplay}
          </Text>
        )
      },
    },
    {
      title: 'Size',
      dataIndex: 'sizeMB',
      key: 'sizeMB',
      width: 100,
      render: (mb: number) => {
        if (mb > 1024) return <Text>{(mb / 1024).toFixed(1)} GB</Text>
        return <Text>{mb} MB</Text>
      },
      sorter: (a: NodeImage, b: NodeImage) => a.sizeMB - b.sizeMB,
    },
    {
      title: '',
      key: 'actions',
      width: 60,
      render: (_: unknown, record: NodeImage) => (
        <Popconfirm
          title="Delete this image from node?"
          description={`This will run crictl rmi on ${nodeName}`}
          onConfirm={() => handleDeleteImage(nodeName, record.image)}
        >
          <Button type="text" size="small" danger icon={<DeleteOutlined />} />
        </Popconfirm>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={5} style={{ margin: 0 }}>{t('menu.images')}</Typography.Title>
        <Space>
          <Input
            prefix={<SearchOutlined />}
            placeholder="Filter images..."
            value={search}
            onChange={e => setSearch(e.target.value)}
            allowClear
            style={{ width: 260 }}
          />
          <Button icon={<ReloadOutlined />} onClick={fetchData} loading={loading}>{t('common.refresh')}</Button>
        </Space>
      </div>

      <Collapse
        defaultActiveKey={data.map(n => n.node)}
        items={filteredData.map(node => ({
          key: node.node,
          label: (
            <Space>
              <CloudServerOutlined />
              <Text strong>{node.node}</Text>
              <Tag>{node.ip}</Tag>
              {node.gpu > 0 && <Tag color="volcano">{node.gpu} GPU</Tag>}
              <Badge count={node.images?.length || 0} style={{ background: '#1677ff' }} overflowCount={99} />
            </Space>
          ),
          children: (
            <Table
              columns={getImageColumns(node.node)}
              dataSource={node.images || []}
              rowKey="image"
              pagination={false}
              size="small"
              locale={{ emptyText: 'No AI images on this node' }}
            />
          ),
        }))}
      />
    </div>
  )
}
