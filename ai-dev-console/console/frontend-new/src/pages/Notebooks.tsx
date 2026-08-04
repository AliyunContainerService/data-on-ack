import React, { useEffect, useState } from 'react';
import {
  Card, Table, Button, Tag, Space, Modal, Form, Input, InputNumber,
  Select, message, Typography, Row, Col, Segmented, Tooltip, Badge, Steps,
  Drawer, Divider, Dropdown,
} from 'antd';
import type { MenuProps } from 'antd';
import {
  PlusOutlined, PlayCircleOutlined, PauseCircleOutlined, DeleteOutlined,
  LinkOutlined, ReloadOutlined, AppstoreOutlined, UnorderedListOutlined,
  FireOutlined, ExperimentOutlined, BarChartOutlined, RobotOutlined,
  PictureOutlined, DatabaseOutlined, CheckCircleOutlined, CodeOutlined,
  CloudServerOutlined, ExpandOutlined, EllipsisOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  listNotebooks, createNotebook, deleteNotebook, stopNotebook, startNotebook,
  getNotebookSSHInfo, resizeNotebook,
  NotebookInfo, NotebookSpec, NotebookSSHInfo, NOTEBOOK_TEMPLATES, RESOURCE_PRESETS, NotebookTemplate,
} from '../api/notebook';

const { Title, Text, Paragraph } = Typography;

const statusConfig: Record<string, { color: string; label: string }> = {
  Running: { color: 'green', label: 'Running' },
  Stopped: { color: 'default', label: 'Stopped' },
  Pending: { color: 'orange', label: 'Starting...' },
  Failed: { color: 'red', label: 'Failed' },
};

const templateIcons: Record<string, React.ReactNode> = {
  fire: <FireOutlined />,
  experiment: <ExperimentOutlined />,
  robot: <RobotOutlined />,
  barChart: <BarChartOutlined />,
  picture: <PictureOutlined />,
  database: <DatabaseOutlined />,
};

const Notebooks: React.FC = () => {
  const { t } = useTranslation();
  const [notebooks, setNotebooks] = useState<NotebookInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [viewMode, setViewMode] = useState<string>('list');
  const [createStep, setCreateStep] = useState(0);
  const [selectedTemplate, setSelectedTemplate] = useState<NotebookTemplate | null>(null);
  const [selectedPreset, setSelectedPreset] = useState<string>('gpu-1');
  const [form] = Form.useForm();

  // SSH & Resize panel
  const [sshDrawerOpen, setSSHDrawerOpen] = useState(false);
  const [sshInfo, setSSHInfo] = useState<NotebookSSHInfo | null>(null);
  const [sshNotebook, setSSHNotebook] = useState<NotebookInfo | null>(null);
  const [resizeModalOpen, setResizeModalOpen] = useState(false);
  const [resizeTarget, setResizeTarget] = useState<NotebookInfo | null>(null);
  const [resizeForm] = Form.useForm();

  const fetchData = () => {
    setLoading(true);
    listNotebooks()
      .then((data) => setNotebooks(data || []))
      .catch(() => setNotebooks([]))
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchData(); }, []);

  const handleTemplateSelect = (template: NotebookTemplate) => {
    setSelectedTemplate(template);
    form.setFieldsValue({
      image: template.image,
      cpu: template.cpu,
      memory: template.memory,
      gpu: template.gpu,
    });
    setCreateStep(1);
  };

  const handlePresetSelect = (presetId: string) => {
    setSelectedPreset(presetId);
    const preset = RESOURCE_PRESETS.find((p) => p.id === presetId);
    if (preset) {
      form.setFieldsValue({
        cpu: preset.cpu,
        memory: preset.memory,
        gpu: preset.gpu,
      });
    }
  };

  const handleCreate = async () => {
    try {
      const values = await form.validateFields();
      const spec: NotebookSpec = {
        name: values.name,
        namespace: values.namespace || 'default',
        image: values.image || selectedTemplate?.image || '',
        cpu: values.cpu || '4',
        memory: values.memory || '16Gi',
        gpu: values.gpu ?? 0,
        storage: values.storage || '50Gi',
        env: selectedTemplate?.env,
      };
      await createNotebook(spec);
      message.success(t('common.success'));
      setCreateModalOpen(false);
      resetCreateModal();
      fetchData();
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const resetCreateModal = () => {
    setCreateStep(0);
    setSelectedTemplate(null);
    setSelectedPreset('gpu-1');
    form.resetFields();
  };

  const handleDelete = async (ns: string, name: string) => {
    try {
      await deleteNotebook(ns, name);
      message.success(t('common.success'));
      fetchData();
    } catch (err) { message.error(err instanceof Error ? err.message : String(err)); }
  };

  const handleStop = async (ns: string, name: string) => {
    try {
      await stopNotebook(ns, name);
      message.success(t('common.success'));
      fetchData();
    } catch (err) { message.error(err instanceof Error ? err.message : String(err)); }
  };

  const handleStart = async (ns: string, name: string) => {
    try {
      await startNotebook(ns, name);
      message.success(t('common.success'));
      fetchData();
    } catch (err) { message.error(err instanceof Error ? err.message : String(err)); }
  };

  const handleSSH = async (record: NotebookInfo) => {
    setSSHNotebook(record);
    setSSHDrawerOpen(true);
    try {
      const info = await getNotebookSSHInfo(record.namespace, record.name);
      setSSHInfo(info);
    } catch {
      setSSHInfo(null);
    }
  };

  const handleResize = (record: NotebookInfo) => {
    setResizeTarget(record);
    resizeForm.setFieldsValue({
      cpu: record.cpu || '4',
      memory: record.memory || '16Gi',
      gpu: parseInt(record.gpu || '0') || 0,
    });
    setResizeModalOpen(true);
  };

  const handleResizeSubmit = async () => {
    if (!resizeTarget) return;
    try {
      const values = await resizeForm.validateFields();
      await resizeNotebook(resizeTarget.namespace, resizeTarget.name, {
        cpu: values.cpu,
        memory: values.memory,
        gpu: values.gpu || 0,
      });
      message.success('Notebook resized. Start it to apply changes.');
      setResizeModalOpen(false);
      fetchData();
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  // Table columns
  const columns = [
    {
      title: t('notebook.name'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: NotebookInfo) => (
        <Space direction="vertical" size={0}>
          <Text strong>{name}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>{record.namespace}</Text>
        </Space>
      ),
    },
    {
      title: t('notebook.status'),
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (status: string) => {
        const cfg = statusConfig[status] || { color: 'default', label: status };
        return (
          <Badge
            status={status === 'Running' ? 'processing' : status === 'Pending' ? 'warning' : status === 'Failed' ? 'error' : 'default'}
            text={cfg.label}
          />
        );
      },
    },
    {
      title: t('notebook.image'),
      dataIndex: 'image',
      key: 'image',
      ellipsis: true,
      width: 220,
      render: (image: string) => (
        <Tooltip title={image}>
          <Tag style={{ maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {image?.split('/').pop()?.split(':')[0] || image}
          </Tag>
        </Tooltip>
      ),
    },
    {
      title: t('notebook.resources'),
      key: 'resources',
      width: 200,
      render: (_: unknown, record: NotebookInfo) => (
        <Space size={4} wrap>
          {record.cpu && <Tag color="blue">{record.cpu} CPU</Tag>}
          {record.memory && <Tag color="purple">{record.memory}</Tag>}
          {record.gpu && record.gpu !== '0' && <Tag color="volcano">{record.gpu} GPU</Tag>}
        </Space>
      ),
    },
    {
      title: t('notebook.age'),
      dataIndex: 'age',
      key: 'age',
      width: 80,
    },
    {
      title: t('notebook.actions'),
      key: 'actions',
      width: 180,
      render: (_: unknown, record: NotebookInfo) => {
        // Build dropdown menu items for secondary actions
        const menuItems: MenuProps['items'] = [];
        if (record.status === 'Running') {
          menuItems.push({ key: 'ssh', icon: <CloudServerOutlined />, label: 'SSH / Terminal', onClick: () => handleSSH(record) });
          menuItems.push({ key: 'stop', icon: <PauseCircleOutlined />, label: t('notebook.stop'), onClick: () => handleStop(record.namespace, record.name) });
        }
        if (record.status === 'Stopped') {
          menuItems.push({ key: 'resize', icon: <ExpandOutlined />, label: 'Resize', onClick: () => handleResize(record) });
        }
        menuItems.push({ type: 'divider' });
        menuItems.push({ key: 'delete', icon: <DeleteOutlined />, label: t('notebook.delete'), danger: true, onClick: () => handleDelete(record.namespace, record.name) });

        return (
          <Space size={4}>
            {/* Primary action */}
            {record.status === 'Running' && record.url && (
              <Button type="primary" size="small" icon={<LinkOutlined />} href={record.url} target="_blank">
                {t('notebook.connect')}
              </Button>
            )}
            {record.status === 'Stopped' && (
              <Button type="primary" ghost size="small" icon={<PlayCircleOutlined />} onClick={() => handleStart(record.namespace, record.name)}>
                {t('notebook.start')}
              </Button>
            )}
            {/* More actions dropdown */}
            <Dropdown menu={{ items: menuItems }} trigger={['click']}>
              <Button size="small" icon={<EllipsisOutlined />} />
            </Dropdown>
          </Space>
        );
      },
    },
  ];

  // Card view for notebooks
  const renderCardView = () => (
    <Row gutter={[16, 16]}>
      {notebooks.map((nb) => (
        <Col xs={24} sm={12} lg={8} xl={6} key={`${nb.namespace}/${nb.name}`}>
          <Card
            className="hover-card"
            size="small"
            style={{ borderRadius: 10 }}
            actions={[
              nb.status === 'Running' && nb.url ? (
                <a href={nb.url} target="_blank" rel="noreferrer"><LinkOutlined /> Open</a>
              ) : null,
              nb.status === 'Running' ? (
                <span onClick={() => handleStop(nb.namespace, nb.name)}><PauseCircleOutlined /> Stop</span>
              ) : nb.status === 'Stopped' ? (
                <span onClick={() => handleStart(nb.namespace, nb.name)}><PlayCircleOutlined /> Start</span>
              ) : null,
            ].filter(Boolean) as React.ReactNode[]}
          >
            <Space direction="vertical" size={8} style={{ width: '100%' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Text strong>{nb.name}</Text>
                <Badge
                  status={nb.status === 'Running' ? 'processing' : nb.status === 'Pending' ? 'warning' : 'default'}
                  text={nb.status}
                />
              </div>
              <Space size={4} wrap>
                {nb.cpu && <Tag color="blue" style={{ fontSize: 11 }}>{nb.cpu} CPU</Tag>}
                {nb.memory && <Tag color="purple" style={{ fontSize: 11 }}>{nb.memory}</Tag>}
                {nb.gpu && nb.gpu !== '0' && <Tag color="volcano" style={{ fontSize: 11 }}>{nb.gpu} GPU</Tag>}
              </Space>
              <Text type="secondary" style={{ fontSize: 11 }} ellipsis>
                {nb.image?.split('/').pop()}
              </Text>
            </Space>
          </Card>
        </Col>
      ))}
      {notebooks.length === 0 && (
        <Col span={24}>
          <div style={{ textAlign: 'center', padding: '60px 0', color: '#9ca3af' }}>
            <CodeOutlined style={{ fontSize: 48, marginBottom: 16, display: 'block' }} />
            <Text type="secondary">No notebooks yet. Create one from a template to get started.</Text>
          </div>
        </Col>
      )}
    </Row>
  );

  return (
    <div>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 20 }}>
        <div>
          <Title level={4} style={{ marginBottom: 4 }}>{t('notebook.title')}</Title>
          <Text type="secondary">{t('notebook.desc')}</Text>
        </div>
        <Space>
          <Segmented
            options={[
              { value: 'list', icon: <UnorderedListOutlined /> },
              { value: 'card', icon: <AppstoreOutlined /> },
            ]}
            value={viewMode}
            onChange={(v) => setViewMode(v as string)}
          />
          <Button icon={<ReloadOutlined />} onClick={fetchData} />
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalOpen(true)}>
            {t('notebook.create')}
          </Button>
        </Space>
      </div>

      {/* Content */}
      {viewMode === 'list' ? (
        <Card bordered={false} style={{ borderRadius: 12 }}>
          <Table
            columns={columns}
            dataSource={notebooks}
            rowKey={(r) => `${r.namespace}/${r.name}`}
            loading={loading}
            pagination={false}
            size="middle"
          />
        </Card>
      ) : (
        renderCardView()
      )}

      {/* Create Modal - Step-based wizard */}
      <Modal
        title={t('notebook.create')}
        open={createModalOpen}
        onCancel={() => { setCreateModalOpen(false); resetCreateModal(); }}
        width={800}
        footer={
          createStep === 0 ? (
            <Button onClick={() => setCreateStep(1)}>Skip templates, configure manually</Button>
          ) : (
            <Space>
              <Button onClick={() => setCreateStep(0)}>Back</Button>
              <Button type="primary" onClick={handleCreate}>Create Notebook</Button>
            </Space>
          )
        }
      >
        <Steps
          current={createStep}
          size="small"
          style={{ marginBottom: 24 }}
          items={[
            { title: t('notebook.template') },
            { title: 'Configure' },
          ]}
        />

        {createStep === 0 ? (
          /* Step 0: Template Gallery */
          <div>
            <Paragraph type="secondary" style={{ marginBottom: 16 }}>
              Choose a pre-configured environment to get started quickly, or skip to configure manually.
            </Paragraph>
            <Row gutter={[12, 12]}>
              {NOTEBOOK_TEMPLATES.map((tmpl) => (
                <Col xs={24} sm={12} key={tmpl.id}>
                  <Card
                    hoverable
                    size="small"
                    style={{
                      borderRadius: 10,
                      border: selectedTemplate?.id === tmpl.id ? '2px solid #1677ff' : undefined,
                    }}
                    onClick={() => handleTemplateSelect(tmpl)}
                  >
                    <div style={{ display: 'flex', gap: 12 }}>
                      <div style={{
                        width: 40, height: 40, borderRadius: 8,
                        background: '#f0f5ff', display: 'flex',
                        alignItems: 'center', justifyContent: 'center',
                        fontSize: 18, color: '#1677ff', flexShrink: 0,
                      }}>
                        {templateIcons[tmpl.icon] || <CodeOutlined />}
                      </div>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <Text strong style={{ fontSize: 13 }}>{tmpl.name}</Text>
                          {selectedTemplate?.id === tmpl.id && <CheckCircleOutlined style={{ color: '#1677ff' }} />}
                        </div>
                        <Text type="secondary" style={{ fontSize: 11, display: 'block', marginTop: 2 }} ellipsis>
                          {tmpl.description}
                        </Text>
                        <Space size={4} style={{ marginTop: 6 }} wrap>
                          {tmpl.tags.map((tag) => (
                            <Tag key={tag} style={{ fontSize: 10, lineHeight: '18px', padding: '0 4px' }}>
                              {tag}
                            </Tag>
                          ))}
                        </Space>
                      </div>
                    </div>
                  </Card>
                </Col>
              ))}
            </Row>
          </div>
        ) : (
          /* Step 1: Configure details */
          <Form form={form} layout="vertical">
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item name="name" label={t('notebook.name')} rules={[{ required: true, pattern: /^[a-z][a-z0-9-]*$/, message: 'Lowercase letters, numbers, hyphens only' }]}>
                  <Input placeholder="my-notebook" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="namespace" label={t('notebook.namespace')} initialValue="default">
                  <Select options={[
                    { label: 'default', value: 'default' },
                    { label: 'kube-ai', value: 'kube-ai' },
                  ]} />
                </Form.Item>
              </Col>
            </Row>

            {selectedTemplate && (
              <div style={{ background: '#f6f8fa', borderRadius: 8, padding: '12px 16px', marginBottom: 16 }}>
                <Text type="secondary" style={{ fontSize: 12 }}>Template: </Text>
                <Text strong style={{ fontSize: 12 }}>{selectedTemplate.name}</Text>
              </div>
            )}

            <Form.Item name="image" label={t('notebook.image')} rules={[{ required: true }]}>
              <Input placeholder="container image" />
            </Form.Item>

            {/* Resource presets */}
            <Form.Item label={t('notebook.resources')}>
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 12 }}>
                {RESOURCE_PRESETS.map((preset) => (
                  <Tag
                    key={preset.id}
                    color={selectedPreset === preset.id ? 'blue' : undefined}
                    style={{ cursor: 'pointer', padding: '4px 10px', borderRadius: 6 }}
                    onClick={() => handlePresetSelect(preset.id)}
                  >
                    {preset.label}
                  </Tag>
                ))}
              </div>
            </Form.Item>

            <Row gutter={16}>
              <Col span={8}>
                <Form.Item name="cpu" label={t('notebook.cpu')} initialValue="4">
                  <Input addonAfter="cores" />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name="memory" label={t('notebook.memory')} initialValue="16Gi">
                  <Input />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name="gpu" label={t('notebook.gpu')} initialValue={0}>
                  <InputNumber min={0} max={8} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
            </Row>

            <Form.Item name="storage" label="Storage" initialValue="50Gi">
              <Select options={[
                { label: '20 GiB', value: '20Gi' },
                { label: '50 GiB', value: '50Gi' },
                { label: '100 GiB', value: '100Gi' },
                { label: '200 GiB', value: '200Gi' },
                { label: '500 GiB', value: '500Gi' },
              ]} />
            </Form.Item>
          </Form>
        )}
      </Modal>

      {/* SSH Access Drawer */}
      <Drawer
        title={<Space><CloudServerOutlined />Terminal Access: {sshNotebook?.name}</Space>}
        open={sshDrawerOpen}
        onClose={() => { setSSHDrawerOpen(false); setSSHInfo(null); }}
        width={520}
      >
        {sshInfo ? (
          <div>
            <div style={{ marginBottom: 20 }}>
              <Text strong style={{ display: 'block', marginBottom: 8 }}>Pod Info</Text>
              <div style={{ background: '#f9f9fb', borderRadius: 8, padding: 12 }}>
                <Row gutter={16}>
                  <Col span={12}><Text type="secondary">Pod:</Text> <Text>{sshInfo.podName}</Text></Col>
                  <Col span={12}><Text type="secondary">Node:</Text> <Text>{sshInfo.node}</Text></Col>
                  <Col span={12}><Text type="secondary">IP:</Text> <Text>{sshInfo.ip}</Text></Col>
                  <Col span={12}><Text type="secondary">Namespace:</Text> <Text>{sshInfo.namespace}</Text></Col>
                </Row>
              </div>
            </div>

            <Divider />

            <div style={{ marginBottom: 20 }}>
              <Text strong style={{ display: 'block', marginBottom: 8 }}>kubectl exec (direct terminal)</Text>
              <Input.TextArea
                value={sshInfo.execCommand}
                readOnly
                rows={2}
                style={{ fontFamily: 'SF Mono, Monaco, Menlo, monospace', fontSize: 12 }}
              />
              <Button size="small" style={{ marginTop: 8 }} onClick={() => { navigator.clipboard.writeText(sshInfo.execCommand); message.success('Copied!'); }}>
                Copy Command
              </Button>
            </div>

            <div style={{ marginBottom: 20 }}>
              <Text strong style={{ display: 'block', marginBottom: 8 }}>SSH via port-forward</Text>
              <Input.TextArea
                value={sshInfo.portForward}
                readOnly
                rows={2}
                style={{ fontFamily: 'SF Mono, Monaco, Menlo, monospace', fontSize: 12 }}
              />
              <Button size="small" style={{ marginTop: 8 }} onClick={() => { navigator.clipboard.writeText(sshInfo.portForward); message.success('Copied!'); }}>
                Copy Command
              </Button>
              <div style={{ marginTop: 8 }}>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  Then connect: ssh -p 2222 root@localhost
                </Text>
              </div>
            </div>
          </div>
        ) : (
          <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>
            Loading pod info...
          </div>
        )}
      </Drawer>

      {/* Resize Modal */}
      <Modal
        title={<Space><ExpandOutlined />Resize: {resizeTarget?.name}</Space>}
        open={resizeModalOpen}
        onOk={handleResizeSubmit}
        onCancel={() => setResizeModalOpen(false)}
        okText="Resize"
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
          Update resource limits for this notebook. Changes will apply on next start.
        </Text>
        <Form form={resizeForm} layout="vertical">
          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="cpu" label="CPU" rules={[{ required: true }]}>
                <Input addonAfter="cores" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="memory" label="Memory" rules={[{ required: true }]}>
                <Input />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="gpu" label="GPU">
                <InputNumber min={0} max={8} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>
    </div>
  );
};

export default Notebooks;
