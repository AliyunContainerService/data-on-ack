import React, { useEffect, useState } from 'react';
import {
  Card, Button, Tag, Space, Typography, Table, Modal, Form, Input, Select,
  message, Popconfirm, Badge,
} from 'antd';
import { PlusOutlined, DatabaseOutlined, ReloadOutlined, DeleteOutlined, ThunderboltOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import PageHeader from '../components/PageHeader';
import EmptyState from '../components/EmptyState';
import { get, post, del } from '../api/client';

const { Text } = Typography;

interface DatasetInfo {
  name: string;
  namespace: string;
  status: string;
  storageClass: string;
  capacity: string;
  accessModes: string[];
  fluidCached: boolean;
  age: string;
}

interface DatasetCreateSpec {
  name: string;
  namespace: string;
  storageClass: string;
  capacity: string;
  accessMode: string;
}

function listDatasets(): Promise<DatasetInfo[]> {
  return get<DatasetInfo[]>('/datasets');
}

function createDataset(spec: DatasetCreateSpec): Promise<unknown> {
  return post('/datasets', spec);
}

function deleteDataset(namespace: string, name: string): Promise<unknown> {
  return del(`/datasets/${namespace}/${name}`);
}

const statusConfig: Record<string, { status: 'processing' | 'success' | 'error' | 'warning' | 'default'; text: string }> = {
  Bound: { status: 'success', text: 'Bound' },
  Pending: { status: 'warning', text: 'Pending' },
  Lost: { status: 'error', text: 'Lost' },
};

const Datasets: React.FC = () => {
  const { t } = useTranslation();
  const [datasets, setDatasets] = useState<DatasetInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();

  const fetchData = () => {
    setLoading(true);
    listDatasets()
      .then((data) => setDatasets(data || []))
      .catch(() => setDatasets([]))
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchData(); }, []);

  const handleCreate = async () => {
    try {
      const values = await form.validateFields();
      const spec: DatasetCreateSpec = {
        name: values.name,
        namespace: values.namespace || 'default',
        storageClass: values.storageClass || '',
        capacity: values.capacity || '100Gi',
        accessMode: values.accessMode || 'ReadWriteOnce',
      };
      await createDataset(spec);
      message.success(t('common.success'));
      setModalOpen(false);
      form.resetFields();
      fetchData();
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const columns = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: DatasetInfo) => (
        <Space direction="vertical" size={0}>
          <Space size={6}>
            <Text strong>{name}</Text>
            {record.fluidCached && (
              <Tag color="cyan" icon={<ThunderboltOutlined />} style={{ fontSize: 10 }}>Fluid Cached</Tag>
            )}
          </Space>
          <Text type="secondary" style={{ fontSize: 11 }}>{record.namespace}</Text>
        </Space>
      ),
    },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      width: 110,
      render: (status: string) => {
        const cfg = statusConfig[status] || { status: 'default' as const, text: status };
        return <Badge status={cfg.status} text={cfg.text} />;
      },
    },
    {
      title: 'Storage Class',
      dataIndex: 'storageClass',
      key: 'storageClass',
      width: 160,
      render: (sc: string) => sc ? <Tag>{sc}</Tag> : <Text type="secondary">-</Text>,
    },
    {
      title: 'Capacity',
      dataIndex: 'capacity',
      key: 'capacity',
      width: 100,
      render: (cap: string) => <Tag color="blue">{cap || '-'}</Tag>,
    },
    {
      title: 'Access Mode',
      dataIndex: 'accessModes',
      key: 'accessModes',
      width: 140,
      render: (modes: string[]) => (
        <Space size={4} wrap>
          {(modes || []).map((m) => <Tag key={m} style={{ fontSize: 10 }}>{m.replace('ReadWrite', 'RW').replace('ReadOnly', 'RO')}</Tag>)}
        </Space>
      ),
    },
    {
      title: 'Actions',
      key: 'actions',
      width: 80,
      render: (_: unknown, record: DatasetInfo) => (
        <Popconfirm title="Delete this PVC?" onConfirm={() => deleteDataset(record.namespace, record.name).then(() => { message.success(t("common.success")); fetchData(); }).catch((err) => message.error(err instanceof Error ? err.message : String(err)))}>
          <Button size="small" danger icon={<DeleteOutlined />} />
        </Popconfirm>
      ),
    },
  ];

  return (
    <div>
      <PageHeader
        title={t('datasets.title')}
        description={t('datasets.desc')}
        extra={
          <>
            <Button icon={<ReloadOutlined />} onClick={fetchData} />
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
              {t('datasets.create')}
            </Button>
          </>
        }
      />

      <Card bordered={false} style={{ borderRadius: 16 }}>
        {datasets.length === 0 && !loading ? (
          <EmptyState
            icon={<DatabaseOutlined />}
            title={t('datasets.empty.title')}
            description={t('datasets.empty.desc')}
            action={
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
                {t('datasets.create')}
              </Button>
            }
          />
        ) : (
          <Table
            columns={columns}
            dataSource={datasets}
            rowKey={(r) => `${r.namespace}/${r.name}`}
            loading={loading}
            pagination={{
              pageSize: 10,
              showSizeChanger: true,
              pageSizeOptions: [10, 20, 50],
              showTotal: (total) => `${total} items`,
            }}
            size="middle"
          />
        )}
      </Card>

      <Modal
        title="Create Persistent Volume Claim"
        open={modalOpen}
        onOk={handleCreate}
        onCancel={() => { setModalOpen(false); form.resetFields(); }}
        okText="Create"
        width={600}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label="PVC Name" rules={[{ required: true, pattern: /^[a-z][a-z0-9-]*$/, message: 'Lowercase, numbers, hyphens' }]}>
            <Input placeholder="my-dataset" />
          </Form.Item>
          <Form.Item name="namespace" label="Namespace" initialValue="default-group">
            <Select options={[
              { label: 'default-group', value: 'default-group' },
              { label: 'default', value: 'default' },
            ]} />
          </Form.Item>
          <Form.Item name="storageClass" label="Storage Class" tooltip="Leave empty for default. Common options: alicloud-disk-essd, alibabacloud-cnfs-nas, alibabacloud-cpfs">
            <Select allowClear placeholder="(cluster default)" options={[
              { label: 'alicloud-disk-essd (ESSD Cloud Disk)', value: 'alicloud-disk-essd' },
              { label: 'alibabacloud-cnfs-nas (NAS)', value: 'alibabacloud-cnfs-nas' },
              { label: 'alibabacloud-cpfs (CPFS)', value: 'alibabacloud-cpfs' },
              { label: 'fluid (Fluid Distributed Cache)', value: 'fluid' },
            ]} />
          </Form.Item>
          <Form.Item name="capacity" label="Capacity" initialValue="100Gi">
            <Select options={[
              { label: '20 GiB', value: '20Gi' },
              { label: '50 GiB', value: '50Gi' },
              { label: '100 GiB', value: '100Gi' },
              { label: '200 GiB', value: '200Gi' },
              { label: '500 GiB', value: '500Gi' },
              { label: '1 TiB', value: '1Ti' },
            ]} />
          </Form.Item>
          <Form.Item name="accessMode" label="Access Mode" initialValue="ReadWriteOnce">
            <Select options={[
              { label: 'ReadWriteOnce (single node)', value: 'ReadWriteOnce' },
              { label: 'ReadWriteMany (multi-node, NAS/CPFS)', value: 'ReadWriteMany' },
              { label: 'ReadOnlyMany (shared read)', value: 'ReadOnlyMany' },
            ]} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Datasets;
