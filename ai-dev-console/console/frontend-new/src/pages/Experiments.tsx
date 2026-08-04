import React, { useEffect, useState } from 'react';
import {
  Card, Table, Button, Tag, Space, Modal, Form, Input, message,
  Popconfirm, Typography, Badge, Drawer, Descriptions, Empty,
} from 'antd';
import {
  PlusOutlined, DeleteOutlined, ReloadOutlined, ExperimentOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  listExperiments, createExperiment, deleteExperiment, getExperiment,
  ExperimentInfo, ExperimentRun, ExperimentCreateSpec,
} from '../api/training';

const { Title, Text } = Typography;

const Experiments: React.FC = () => {
  const { t } = useTranslation();
  const [experiments, setExperiments] = useState<ExperimentInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [form] = Form.useForm();

  // Detail drawer
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailExp, setDetailExp] = useState<ExperimentInfo | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const fetchData = () => {
    setLoading(true);
    listExperiments()
      .then((data) => setExperiments(data || []))
      .catch(() => setExperiments([]))
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchData(); }, []);

  const handleCreate = async () => {
    try {
      const values = await form.validateFields();
      const spec: ExperimentCreateSpec = {
        name: values.name,
        namespace: values.namespace || 'default-group',
        description: values.description || '',
      };
      setSubmitting(true);
      await createExperiment(spec);
      message.success(t('common.success'));
      setCreateOpen(false);
      form.resetFields();
      fetchData();
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (name: string, namespace: string) => {
    try {
      await deleteExperiment(name, namespace);
      message.success(t('common.success'));
      fetchData();
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const handleViewDetail = async (record: ExperimentInfo) => {
    setDetailOpen(true);
    setDetailLoading(true);
    try {
      const exp = await getExperiment(record.name, record.namespace);
      setDetailExp(exp);
    } catch {
      setDetailExp(record);
    } finally {
      setDetailLoading(false);
    }
  };

  const columns = [
    {
      title: t('experiment.name'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: ExperimentInfo) => (
        <Button type="link" style={{ padding: 0, fontWeight: 500 }} onClick={() => handleViewDetail(record)}>
          {name}
        </Button>
      ),
    },
    {
      title: t('experiment.description'),
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: t('experiment.runs'),
      dataIndex: 'runCount',
      key: 'runCount',
      width: 80,
      render: (count: number) => <Badge count={count} showZero style={{ backgroundColor: '#52c41a' }} />,
    },
    {
      title: t('experiment.bestLoss'),
      dataIndex: 'bestLoss',
      key: 'bestLoss',
      width: 120,
      render: (v: number) => v ? <Text strong style={{ color: '#0071e3' }}>{v.toFixed(4)}</Text> : '-',
    },
    {
      title: t('experiment.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      width: 120,
      render: (ns: string) => <Tag>{ns}</Tag>,
    },
    {
      title: t('experiment.createTime'),
      dataIndex: 'createTime',
      key: 'createTime',
      width: 160,
      render: (time: string) => time ? new Date(time).toLocaleString() : '-',
    },
    {
      title: '',
      key: 'actions',
      width: 60,
      render: (_: unknown, record: ExperimentInfo) => (
        <Popconfirm title={t('common.delete.confirm')} onConfirm={() => handleDelete(record.name, record.namespace)}>
          <Button type="text" size="small" danger icon={<DeleteOutlined />} />
        </Popconfirm>
      ),
    },
  ];

  // Status color for run status
  const runStatusColor = (status: string) => {
    switch (status) {
      case 'Running': return 'processing';
      case 'Succeeded': return 'success';
      case 'Failed': return 'error';
      default: return 'warning';
    }
  };

  return (
    <div style={{ padding: '0 4px' }}>
      <div style={{ marginBottom: 20 }}>
        <Title level={4} style={{ marginBottom: 4 }}>{t('experiment.title')}</Title>
        <Text type="secondary">{t('experiment.desc')}</Text>
      </div>

      <Card
        styles={{ body: { padding: 0 } }}
        style={{ borderRadius: 14, border: 'none', boxShadow: '0 1px 3px rgba(0,0,0,0.04)' }}
        title={
          <Space>
            <ExperimentOutlined />
            <span>{t('experiment.title')}</span>
            <Badge count={experiments.length} style={{ backgroundColor: '#f0f0f0', color: '#666' }} />
          </Space>
        }
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={fetchData}>{t('common.refresh')}</Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
              {t('experiment.create')}
            </Button>
          </Space>
        }
      >
        <Table
          columns={columns}
          dataSource={experiments}
          loading={loading}
          rowKey={(r) => `${r.namespace}/${r.name}`}
          pagination={{ pageSize: 15, showSizeChanger: false }}
          locale={{ emptyText: <Empty description={t('common.nodata')} /> }}
        />
      </Card>

      {/* Create Experiment Modal */}
      <Modal
        title={<Space><ExperimentOutlined />{t('experiment.create')}</Space>}
        open={createOpen}
        onOk={handleCreate}
        confirmLoading={submitting}
        onCancel={() => { setCreateOpen(false); form.resetFields(); }}
        okText={t('common.confirm')}
        cancelText={t('common.cancel')}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('experiment.name')} rules={[{ required: true, pattern: /^[a-z][a-z0-9-]*$/, message: 'lowercase, numbers, hyphens' }]}>
            <Input placeholder="my-experiment" />
          </Form.Item>
          <Form.Item name="namespace" label={t('experiment.namespace')} initialValue="default-group">
            <Input />
          </Form.Item>
          <Form.Item name="description" label={t('experiment.description')}>
            <Input.TextArea rows={3} placeholder="Experiment description..." />
          </Form.Item>
        </Form>
      </Modal>

      {/* Experiment Detail Drawer */}
      <Drawer
        title={<Space><ExperimentOutlined />{detailExp?.name}</Space>}
        open={detailOpen}
        onClose={() => { setDetailOpen(false); setDetailExp(null); }}
        width={700}
        styles={{ body: { padding: '16px 20px' } }}
      >
        {detailLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>Loading...</div>
        ) : detailExp ? (
          <div>
            <Descriptions column={2} style={{ marginBottom: 20 }}>
              <Descriptions.Item label={t('experiment.name')}>{detailExp.name}</Descriptions.Item>
              <Descriptions.Item label={t('experiment.namespace')}>{detailExp.namespace}</Descriptions.Item>
              <Descriptions.Item label={t('experiment.description')} span={2}>{detailExp.description || '-'}</Descriptions.Item>
              <Descriptions.Item label={t('experiment.runs')}>{detailExp.runCount}</Descriptions.Item>
              <Descriptions.Item label={t('experiment.bestLoss')}>
                {detailExp.bestLoss ? detailExp.bestLoss.toFixed(4) : '-'}
              </Descriptions.Item>
            </Descriptions>

            {/* Runs comparison table */}
            <Title level={5} style={{ marginBottom: 12 }}>{t('experiment.compare')}</Title>
            {detailExp.runs && detailExp.runs.length > 0 ? (
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', fontSize: 12, borderCollapse: 'collapse', border: '1px solid #f0f0f0', borderRadius: 8 }}>
                  <thead>
                    <tr style={{ background: '#f9f9fb' }}>
                      <th style={{ padding: '10px 12px', textAlign: 'left', borderBottom: '1px solid #f0f0f0' }}>Job</th>
                      <th style={{ padding: '10px 12px', textAlign: 'left', borderBottom: '1px solid #f0f0f0' }}>Kind</th>
                      <th style={{ padding: '10px 12px', textAlign: 'left', borderBottom: '1px solid #f0f0f0' }}>Status</th>
                      <th style={{ padding: '10px 12px', textAlign: 'left', borderBottom: '1px solid #f0f0f0' }}>{t('experiment.params')}</th>
                      <th style={{ padding: '10px 12px', textAlign: 'left', borderBottom: '1px solid #f0f0f0' }}>{t('experiment.metrics')}</th>
                      <th style={{ padding: '10px 12px', textAlign: 'left', borderBottom: '1px solid #f0f0f0' }}>Added</th>
                    </tr>
                  </thead>
                  <tbody>
                    {detailExp.runs.map((run: ExperimentRun, idx: number) => (
                      <tr key={idx} style={{ borderBottom: '1px solid #f0f0f0' }}>
                        <td style={{ padding: '8px 12px', fontFamily: 'monospace', fontSize: 11 }}>{run.jobName}</td>
                        <td style={{ padding: '8px 12px' }}><Tag>{run.jobKind}</Tag></td>
                        <td style={{ padding: '8px 12px' }}>
                          <Badge status={runStatusColor(run.status) as 'processing' | 'success' | 'error' | 'warning'} text={run.status} />
                        </td>
                        <td style={{ padding: '8px 12px' }}>
                          {run.params ? Object.entries(run.params).map(([k, v]) => (
                            <Tag key={k} style={{ fontSize: 10, marginBottom: 2 }}>{k}={v}</Tag>
                          )) : '-'}
                        </td>
                        <td style={{ padding: '8px 12px' }}>
                          {run.metrics ? Object.entries(run.metrics).map(([k, v]) => (
                            <Tag key={k} color="blue" style={{ fontSize: 10, marginBottom: 2 }}>{k}: {v.toFixed(4)}</Tag>
                          )) : '-'}
                        </td>
                        <td style={{ padding: '8px 12px', fontSize: 11 }}>
                          {run.addedAt ? new Date(run.addedAt).toLocaleDateString() : '-'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <Empty description={t('experiment.noRuns')} />
            )}
          </div>
        ) : null}
      </Drawer>
    </div>
  );
};

export default Experiments;
