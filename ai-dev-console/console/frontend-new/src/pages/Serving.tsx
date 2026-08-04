import React, { useEffect, useState } from 'react';
import {
  Card, Table, Button, Tag, Space, Modal, Form, Input, InputNumber,
  Select, message, Popconfirm, Typography, Row, Col, Badge, Alert,
  Segmented, Switch, Divider, Drawer, Spin, Slider, Tabs,
} from 'antd';
import { PlusOutlined, DeleteOutlined, ReloadOutlined, ApiOutlined, ThunderboltOutlined, RocketOutlined, SendOutlined, FileTextOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  listServing, createServing, deleteServing, testServingEndpoint,
  listServingPods, getServingLogs,
  ServingInfo, ServingSpec, INFERENCE_ENGINES, ChatMessage, PodInfo,
} from '../api/serving';

const { Title, Text } = Typography;

const statusConfig: Record<string, { status: 'processing' | 'success' | 'error' | 'warning' | 'default'; text: string }> = {
  Ready: { status: 'success', text: 'Ready' },
  Pending: { status: 'warning', text: 'Starting...' },
  Progressing: { status: 'processing', text: 'Progressing' },
  NotReady: { status: 'error', text: 'Not Ready' },
};

type DeployMode = 'single' | 'distributed' | 'pd-split';

const Serving: React.FC = () => {
  const { t } = useTranslation();
  const [services, setServices] = useState<ServingInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [selectedEngine, setSelectedEngine] = useState<string>('vllm');
  const [deployMode, setDeployMode] = useState<DeployMode>('single');
  const [form] = Form.useForm();

  // Serving logs state
  const [logDrawerOpen, setLogDrawerOpen] = useState(false);
  const [logService, setLogService] = useState<ServingInfo | null>(null);
  const [logPods, setLogPods] = useState<PodInfo[]>([]);
  const [selectedLogPod, setSelectedLogPod] = useState<string>('');
  const [logContent, setLogContent] = useState<string>('');
  const [logLoading, setLogLoading] = useState(false);

  // Chat test panel state
  const [testDrawerOpen, setTestDrawerOpen] = useState(false);
  const [testService, setTestService] = useState<ServingInfo | null>(null);
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([]);
  const [chatInput, setChatInput] = useState('');
  const [chatLoading, setChatLoading] = useState(false);
  const [temperature, setTemperature] = useState(0.7);
  const [maxTokens, setMaxTokens] = useState(1024);

  const fetchData = () => {
    setLoading(true);
    listServing()
      .then((data) => setServices(data || []))
      .catch(() => setServices([]))
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchData(); }, []);

  const handleTest = (record: ServingInfo) => {
    setTestService(record);
    setChatMessages([]);
    setChatInput('');
    setTestDrawerOpen(true);
  };

  const handleViewLogs = async (record: ServingInfo) => {
    setLogService(record);
    setLogDrawerOpen(true);
    setLogContent('');
    setSelectedLogPod('');
    try {
      const pods = await listServingPods(record.namespace, record.name);
      setLogPods(pods || []);
      if (pods && pods.length > 0) {
        setSelectedLogPod(pods[0].name);
        fetchServingLogs(record.namespace, record.name, pods[0].name);
      }
    } catch {
      setLogPods([]);
    }
  };

  const fetchServingLogs = async (namespace: string, name: string, podName: string) => {
    setLogLoading(true);
    try {
      const logs = await getServingLogs(namespace, name, podName, 1000);
      setLogContent(logs || '(no logs)');
    } catch (err) {
      setLogContent(err instanceof Error ? `Error: ${err.message}` : 'Failed to fetch logs');
    } finally {
      setLogLoading(false);
    }
  };

  const handleLogPodChange = (podName: string) => {
    setSelectedLogPod(podName);
    if (logService) {
      fetchServingLogs(logService.namespace, logService.name, podName);
    }
  };

  const handleSendMessage = async () => {
    if (!chatInput.trim() || !testService) return;
    const userMsg: ChatMessage = { role: 'user', content: chatInput.trim() };
    const updatedMessages = [...chatMessages, userMsg];
    setChatMessages(updatedMessages);
    setChatInput('');
    setChatLoading(true);

    try {
      const result = await testServingEndpoint(testService.namespace, testService.name, {
        messages: updatedMessages,
        temperature,
        max_tokens: maxTokens,
      });
      // Extract assistant response from OpenAI-compatible format
      let assistantContent = '';
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const choices = (result as any)?.choices;
      if (choices && choices.length > 0) {
        assistantContent = choices[0]?.message?.content || choices[0]?.text || JSON.stringify(choices[0]);
      } else if ((result as Record<string, unknown>).raw) {
        assistantContent = (result as Record<string, unknown>).raw as string;
      } else {
        assistantContent = JSON.stringify(result, null, 2);
      }
      setChatMessages([...updatedMessages, { role: 'assistant', content: assistantContent }]);
    } catch (err) {
      const errMsg = err instanceof Error ? err.message : 'Request failed';
      setChatMessages([...updatedMessages, { role: 'assistant', content: `Error: ${errMsg}` }]);
    } finally {
      setChatLoading(false);
    }
  };

  const handleEngineChange = (engineId: string) => {
    setSelectedEngine(engineId);
    const engine = INFERENCE_ENGINES.find((e) => e.id === engineId);
    if (engine) {
      form.setFieldsValue({
        image: engine.defaultImage,
        port: engine.defaultPort,
        framework: engineId,
      });
    }
  };

  const handleCreate = async () => {
    try {
      const values = await form.validateFields();
      const spec: ServingSpec = {
        name: values.name,
        namespace: values.namespace || 'default-group',
        image: values.image,
        command: values.command,
        replicas: values.replicas || 1,
        cpu: values.cpu || '4',
        memory: values.memory || '16Gi',
        gpu: values.gpu || 1,
        port: values.port || 8000,
        framework: values.framework || selectedEngine,
        modelPath: values.modelPath,
        env: values.modelName ? { MODEL_NAME: values.modelName } : undefined,
      };
      await createServing(spec);
      message.success(t('common.success'));
      setCreateOpen(false);
      form.resetFields();
      fetchData();
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const currentEngine = INFERENCE_ENGINES.find((e) => e.id === selectedEngine);

  const columns = [
    {
      title: t('serving.name'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: ServingInfo) => (
        <Space direction="vertical" size={0}>
          <Text strong>{name}</Text>
          <Text type="secondary" style={{ fontSize: 11 }}>{record.namespace}</Text>
        </Space>
      ),
    },
    {
      title: t('serving.framework'),
      dataIndex: 'framework',
      key: 'framework',
      width: 140,
      render: (fw: string) => {
        const colors: Record<string, string> = { vllm: 'purple', sglang: 'blue', tgi: 'orange', custom: 'default' };
        return <Tag color={colors[fw] || 'default'}>{fw?.toUpperCase() || 'Custom'}</Tag>;
      },
    },
    {
      title: t('serving.status'),
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (status: string) => {
        const cfg = statusConfig[status] || { status: 'default' as const, text: status };
        return <Badge status={cfg.status} text={cfg.text} />;
      },
    },
    {
      title: t('serving.replicas'),
      dataIndex: 'replicas',
      key: 'replicas',
      width: 90,
    },
    {
      title: t('serving.endpoint'),
      dataIndex: 'endpoint',
      key: 'endpoint',
      ellipsis: true,
      render: (ep: string) => ep ? <Text copyable style={{ fontSize: 12 }}>{ep}</Text> : <Text type="secondary">-</Text>,
    },
    {
      title: t('serving.actions'),
      key: 'actions',
      width: 220,
      render: (_: unknown, record: ServingInfo) => (
        <Space size={4}>
          <Button size="small" icon={<ApiOutlined />} onClick={() => handleTest(record)}>{t('serving.test')}</Button>
          <Button size="small" icon={<FileTextOutlined />} onClick={() => handleViewLogs(record)}>Logs</Button>
          <Popconfirm title={t('common.delete.confirm')} onConfirm={() => deleteServing(record.namespace, record.name).then(() => { message.success(t("common.success")); fetchData(); }).catch((err) => message.error(err instanceof Error ? err.message : String(err)))}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 20 }}>
        <div>
          <Title level={4} style={{ marginBottom: 4 }}>{t('serving.title')}</Title>
          <Text type="secondary">{t('serving.desc')}</Text>
        </div>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchData} />
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            {t('serving.create')}
          </Button>
        </Space>
      </div>

      <Card bordered={false} style={{ borderRadius: 14 }}>
        <Table columns={columns} dataSource={services} rowKey={(r) => `${r.namespace}/${r.name}`} loading={loading} pagination={false} size="middle" />
      </Card>

      {/* Serving Logs Drawer */}
      <Drawer
        title={<Space><FileTextOutlined />Logs: {logService?.name}</Space>}
        open={logDrawerOpen}
        onClose={() => { setLogDrawerOpen(false); setLogService(null); setLogPods([]); setLogContent(''); }}
        width={680}
        styles={{ body: { padding: 0 } }}
      >
        <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
          {logPods.length > 0 && (
            <Tabs
              activeKey={selectedLogPod}
              onChange={handleLogPodChange}
              style={{ padding: '0 16px' }}
              items={logPods.map((p) => ({
                key: p.name,
                label: (
                  <Space size={4}>
                    <Badge status={p.status === 'Running' ? 'processing' : 'default'} />
                    <span>{p.name.split('-').slice(-2).join('-')}</span>
                  </Space>
                ),
              }))}
            />
          )}
          {logPods.length === 0 && (
            <div style={{ padding: 16, color: '#999' }}>No pods found for this service.</div>
          )}
          <div style={{ flex: 1, overflow: 'auto', padding: 16 }}>
            {logLoading ? (
              <div style={{ textAlign: 'center', padding: 40 }}><Spin tip="Loading logs..." /></div>
            ) : (
              <pre style={{
                background: '#1e1e1e', color: '#d4d4d4', padding: 16, borderRadius: 8,
                fontSize: 12, fontFamily: 'SF Mono, Monaco, Menlo, monospace',
                lineHeight: 1.6, whiteSpace: 'pre-wrap', wordBreak: 'break-all',
                minHeight: 400, margin: 0,
              }}>
                {logContent}
              </pre>
            )}
          </div>
        </div>
      </Drawer>

      {/* Chat Test Drawer */}
      <Drawer
        title={<Space><ApiOutlined />API Playground: {testService?.name}</Space>}
        open={testDrawerOpen}
        onClose={() => { setTestDrawerOpen(false); setTestService(null); }}
        width={560}
        styles={{ body: { padding: 0, display: 'flex', flexDirection: 'column', height: '100%' } }}
      >
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', height: '100%' }}>
          {/* Settings bar */}
          <div style={{ padding: '12px 16px', borderBottom: '1px solid #f0f0f0', background: '#f9f9fb' }}>
            <Row gutter={16} align="middle">
              <Col span={12}>
                <Text style={{ fontSize: 11 }}>Temperature: {temperature}</Text>
                <Slider min={0} max={2} step={0.1} value={temperature} onChange={setTemperature} />
              </Col>
              <Col span={12}>
                <Text style={{ fontSize: 11 }}>Max Tokens: {maxTokens}</Text>
                <Slider min={64} max={4096} step={64} value={maxTokens} onChange={setMaxTokens} />
              </Col>
            </Row>
          </div>

          {/* Chat messages */}
          <div style={{ flex: 1, overflow: 'auto', padding: 16 }}>
            {chatMessages.length === 0 && (
              <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>
                <ApiOutlined style={{ fontSize: 32, marginBottom: 8 }} />
                <div>Send a message to test the inference endpoint</div>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  Uses OpenAI-compatible /v1/chat/completions API
                </Text>
              </div>
            )}
            {chatMessages.map((msg, idx) => (
              <div key={idx} style={{
                marginBottom: 12,
                display: 'flex',
                justifyContent: msg.role === 'user' ? 'flex-end' : 'flex-start',
              }}>
                <div style={{
                  maxWidth: '80%',
                  padding: '10px 14px',
                  borderRadius: msg.role === 'user' ? '14px 14px 4px 14px' : '14px 14px 14px 4px',
                  background: msg.role === 'user' ? '#0071e3' : '#f0f0f0',
                  color: msg.role === 'user' ? '#fff' : '#1d1d1f',
                  fontSize: 13,
                  lineHeight: 1.5,
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                }}>
                  {msg.content}
                </div>
              </div>
            ))}
            {chatLoading && (
              <div style={{ display: 'flex', justifyContent: 'flex-start', marginBottom: 12 }}>
                <div style={{ padding: '10px 14px', borderRadius: '14px 14px 14px 4px', background: '#f0f0f0' }}>
                  <Spin size="small" />
                </div>
              </div>
            )}
          </div>

          {/* Input area */}
          <div style={{ padding: '12px 16px', borderTop: '1px solid #f0f0f0', background: '#fff' }}>
            <Space.Compact style={{ width: '100%' }}>
              <Input.TextArea
                value={chatInput}
                onChange={(e) => setChatInput(e.target.value)}
                placeholder="Type a message..."
                autoSize={{ minRows: 1, maxRows: 4 }}
                onPressEnter={(e) => {
                  if (!e.shiftKey) {
                    e.preventDefault();
                    handleSendMessage();
                  }
                }}
                style={{ borderRadius: '10px 0 0 10px' }}
              />
              <Button
                type="primary"
                icon={<SendOutlined />}
                onClick={handleSendMessage}
                loading={chatLoading}
                style={{ borderRadius: '0 10px 10px 0', height: 'auto' }}
              />
            </Space.Compact>
          </div>
        </div>
      </Drawer>

      {/* Create Serving - Full Professional Form */}
      <Modal
        title={<Space><RocketOutlined />{t('serving.create')}</Space>}
        open={createOpen}
        onOk={handleCreate}
        onCancel={() => { setCreateOpen(false); form.resetFields(); }}
        okText={t('common.confirm')}
        cancelText={t('common.cancel')}
        width={780}
        styles={{ body: { maxHeight: '70vh', overflowY: 'auto' } }}
      >
        {/* Deployment Mode */}
        <div style={{ background: '#f9f9fb', borderRadius: 12, padding: '16px 20px', marginBottom: 16 }}>
          <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 12 }}>Deployment Mode</Text>
          <Segmented
            block
            options={[
              { label: 'Single Node (StatefulSet)', value: 'single' },
              { label: 'Multi-Node Distributed (LeaderWorkerSet)', value: 'distributed' },
              { label: 'PD Disaggregation (RoleBasedGroup)', value: 'pd-split' },
            ]}
            value={deployMode}
            onChange={(v) => setDeployMode(v as DeployMode)}
          />
          {deployMode === 'distributed' && (
            <Alert type="info" showIcon style={{ marginTop: 12, borderRadius: 8 }}
              message="Multi-node distributed inference uses LeaderWorkerSet to coordinate a leader + multiple workers with tensor/pipeline parallelism across nodes." />
          )}
          {deployMode === 'pd-split' && (
            <Alert type="info" showIcon style={{ marginTop: 12, borderRadius: 8 }}
              message="PD Disaggregation separates Prefill and Decode phases into independent scalable roles. Supports SGLang PD, Dynamo PD architectures." />
          )}
        </div>

        {/* Engine Selection */}
        <div style={{ background: '#f9f9fb', borderRadius: 12, padding: '16px 20px', marginBottom: 16 }}>
          <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 12 }}>{t('serving.framework')}</Text>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            {INFERENCE_ENGINES.map((engine) => (
              <Card
                key={engine.id}
                size="small"
                hoverable
                style={{
                  width: 150, borderRadius: 10,
                  border: selectedEngine === engine.id ? '2px solid #0071e3' : '1px solid #e8e8ed',
                }}
                onClick={() => handleEngineChange(engine.id)}
              >
                <Text strong style={{ fontSize: 12 }}>{engine.name}</Text>
                <div><Text type="secondary" style={{ fontSize: 10 }}>{engine.description.slice(0, 35)}...</Text></div>
              </Card>
            ))}
          </div>
        </div>

        {currentEngine && (
          <Alert type="info" showIcon icon={<ThunderboltOutlined />}
            message={currentEngine.name} description={currentEngine.description}
            style={{ marginBottom: 16, borderRadius: 10 }} />
        )}

        <Form form={form} layout="vertical" initialValues={{ framework: 'vllm', port: 8000 }}>
          {/* Basic Info */}
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label={t('serving.name')} rules={[{ required: true }]}>
                <Input placeholder="qwen2-7b-serving" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="namespace" label="Namespace" initialValue="default-group">
                <Select options={[
                  { label: 'default-group', value: 'default-group' },
                  { label: 'default', value: 'default' },
                ]} />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="image" label="Image" rules={[{ required: true }]}>
            <Input placeholder={currentEngine?.defaultImage} />
          </Form.Item>

          {/* Model Configuration */}
          <div style={{ background: '#f9f9fb', borderRadius: 12, padding: '16px 20px', marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 12 }}>Model</Text>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item name="modelPath" label="Model Path (PVC or OSS)">
                  <Input placeholder="/models/Qwen2-7B-Instruct or oss://bucket/models/" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="modelName" label="Model Name / HuggingFace ID">
                  <Input placeholder="Qwen/Qwen2-7B-Instruct" />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item name="fluidCache" label="Fluid Cache Acceleration" valuePropName="checked" tooltip="Use Fluid distributed cache to accelerate model loading (requires Fluid Dataset)">
              <Switch />
            </Form.Item>
          </div>

          {/* Command */}
          <Form.Item name="command" label="Startup Command (optional)">
            <Input.TextArea rows={2} placeholder={
              selectedEngine === 'vllm' ? 'python -m vllm.entrypoints.openai.api_server --model $MODEL_NAME --tensor-parallel-size $TP_SIZE' :
              selectedEngine === 'sglang' ? 'python -m sglang.launch_server --model-path $MODEL_PATH --tp $TP_SIZE --port 30000' :
              'Optional: override default entrypoint'
            } style={{ fontFamily: 'monospace', fontSize: 12 }} />
          </Form.Item>

          {/* Resources */}
          <div style={{ background: '#f9f9fb', borderRadius: 12, padding: '16px 20px', marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 12 }}>Resources</Text>
            <Row gutter={16}>
              <Col span={6}>
                <Form.Item name="replicas" label="Replicas" initialValue={1}>
                  <InputNumber min={1} max={32} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={6}>
                <Form.Item name="cpu" label="CPU" initialValue="8">
                  <Input addonAfter="cores" />
                </Form.Item>
              </Col>
              <Col span={6}>
                <Form.Item name="memory" label="Memory" initialValue="32Gi">
                  <Select options={[
                    { label: '16 GiB', value: '16Gi' },
                    { label: '32 GiB', value: '32Gi' },
                    { label: '64 GiB', value: '64Gi' },
                    { label: '128 GiB', value: '128Gi' },
                  ]} />
                </Form.Item>
              </Col>
              <Col span={6}>
                <Form.Item name="gpu" label="GPU" initialValue={1}>
                  <InputNumber min={0} max={8} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
            </Row>

            {deployMode === 'distributed' && (
              <Row gutter={16}>
                <Col span={8}>
                  <Form.Item name="workerNodes" label="Worker Nodes" initialValue={2} tooltip="Number of worker nodes for tensor/pipeline parallelism">
                    <InputNumber min={2} max={32} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item name="tpSize" label="Tensor Parallel Size" initialValue={1}>
                    <InputNumber min={1} max={8} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item name="ppSize" label="Pipeline Parallel Size" initialValue={2}>
                    <InputNumber min={1} max={32} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
              </Row>
            )}

            {deployMode === 'pd-split' && (
              <>
                <Divider style={{ margin: '12px 0' }} />
                <Text strong style={{ fontSize: 12, color: '#6e6e73' }}>PD Disaggregation Roles</Text>
                <Row gutter={16} style={{ marginTop: 8 }}>
                  <Col span={8}>
                    <Form.Item name="prefillReplicas" label="Prefill Replicas" initialValue={2}>
                      <InputNumber min={1} max={16} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                  <Col span={8}>
                    <Form.Item name="decodeReplicas" label="Decode Replicas" initialValue={4}>
                      <InputNumber min={1} max={32} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                  <Col span={8}>
                    <Form.Item name="schedulerReplicas" label="Scheduler" initialValue={1}>
                      <InputNumber min={1} max={2} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                </Row>
              </>
            )}
          </div>

          {/* Autoscaling */}
          <div style={{ background: '#f9f9fb', borderRadius: 12, padding: '16px 20px', marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 12 }}>Autoscaling (HPA)</Text>
            <Form.Item name="autoscaleEnabled" valuePropName="checked" label="Enable Autoscaling">
              <Switch />
            </Form.Item>
            <Row gutter={16}>
              <Col span={8}>
                <Form.Item name="minReplicas" label="Min Replicas" initialValue={1}>
                  <InputNumber min={1} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name="maxReplicas" label="Max Replicas" initialValue={4}>
                  <InputNumber min={1} max={64} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name="scaleMetric" label="Scale Metric" initialValue="gpu-utilization">
                  <Select options={[
                    { label: 'GPU Utilization', value: 'gpu-utilization' },
                    { label: 'Request Queue Depth', value: 'queue-depth' },
                    { label: 'KV Cache Usage', value: 'kv-cache' },
                    { label: 'Tokens/Second', value: 'throughput' },
                  ]} />
                </Form.Item>
              </Col>
            </Row>
          </div>

          <Form.Item name="port" label="Port" hidden><InputNumber /></Form.Item>
          <Form.Item name="framework" hidden><Input /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Serving;
