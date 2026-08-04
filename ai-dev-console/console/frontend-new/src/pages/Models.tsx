import React, { useEffect, useState } from 'react';
import {
  Card, Button, Tag, Space, Typography, Row, Col, Input, Empty, Modal, Form, Select,
  message, Popconfirm,
} from 'antd';
import { PlusOutlined, RocketOutlined, SearchOutlined, AppstoreOutlined, DeleteOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { get, post, del } from '../api/client';

const { Title, Text, Paragraph } = Typography;

interface ModelInfo {
  name: string;
  namespace: string;
  version: string;
  framework: string;
  path: string;
  source: string;
  size: string;
  registerTime: string;
}

interface ModelRegisterSpec {
  name: string;
  namespace: string;
  version: string;
  framework: string;
  path: string;
  source: string;
  size: string;
}

function listModels(): Promise<ModelInfo[]> {
  return get<ModelInfo[]>('/models');
}

function registerModel(spec: ModelRegisterSpec): Promise<unknown> {
  return post('/models', spec);
}

function deleteModel(namespace: string, name: string): Promise<unknown> {
  return del(`/models/${namespace}/${name}`);
}

const frameworkColors: Record<string, string> = {
  pytorch: 'blue',
  tensorflow: 'orange',
  onnx: 'green',
  safetensors: 'purple',
  custom: 'default',
};

const sourceLabels: Record<string, string> = {
  huggingface: 'HuggingFace',
  modelscope: 'ModelScope',
  training: 'Training Output',
  custom: 'Custom',
};

const Models: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [registerOpen, setRegisterOpen] = useState(false);
  const [form] = Form.useForm();

  const fetchData = () => {
    setLoading(true);
    listModels()
      .then((data) => setModels(data || []))
      .catch(() => setModels([]))
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchData(); }, []);

  const handleRegister = async () => {
    try {
      const values = await form.validateFields();
      await registerModel(values as ModelRegisterSpec);
      message.success('Model registered');
      setRegisterOpen(false);
      form.resetFields();
      fetchData();
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    }
  };

  const handleDeploy = (model: ModelInfo) => {
    // Navigate to Serving page with pre-filled model info
    navigate('/serving', { state: { deployModel: model } });
  };

  const filteredModels = models.filter((m) =>
    m.name.toLowerCase().includes(search.toLowerCase()) ||
    m.framework.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 20 }}>
        <div>
          <Title level={4} style={{ marginBottom: 4 }}>{t('models.title')}</Title>
          <Text type="secondary">{t('models.desc')}</Text>
        </div>
        <Space>
          <Input
            prefix={<SearchOutlined />}
            placeholder={t('common.search')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ width: 220 }}
            allowClear
          />
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setRegisterOpen(true)}>
            {t('models.register')}
          </Button>
        </Space>
      </div>

      {filteredModels.length === 0 && !loading ? (
        <Card bordered={false} style={{ borderRadius: 12 }}>
          <Empty
            image={<AppstoreOutlined style={{ fontSize: 64, color: '#d1d5db' }} />}
            imageStyle={{ height: 80 }}
            description={
              <Space direction="vertical" size={4}>
                <Text type="secondary">No models registered yet</Text>
                <Paragraph type="secondary" style={{ fontSize: 13 }}>
                  Register a model from training output, HuggingFace Hub, or ModelScope to manage versions and deploy to inference.
                </Paragraph>
              </Space>
            }
          >
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setRegisterOpen(true)}>
              Register Model
            </Button>
          </Empty>
        </Card>
      ) : (
        <Row gutter={[16, 16]}>
          {filteredModels.map((model) => (
            <Col xs={24} sm={12} lg={8} key={`${model.namespace}/${model.name}`}>
              <Card className="hover-card" bordered={false} style={{ borderRadius: 12 }}>
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <Text strong style={{ fontSize: 14 }}>{model.name}</Text>
                    <Tag color="blue">v{model.version || '1.0'}</Tag>
                  </div>
                  <Space size={4} wrap>
                    <Tag color={frameworkColors[model.framework] || 'default'}>{model.framework}</Tag>
                    {model.size && <Tag color="volcano">{model.size}</Tag>}
                    <Tag>{sourceLabels[model.source] || model.source}</Tag>
                  </Space>
                  <Text type="secondary" style={{ fontSize: 11, display: 'block' }} ellipsis>
                    {model.path}
                  </Text>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 4 }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{model.namespace} | {model.registerTime}</Text>
                    <Space size={4}>
                      <Button type="primary" size="small" icon={<RocketOutlined />} onClick={() => handleDeploy(model)}>
                        Deploy
                      </Button>
                      <Popconfirm title="Remove this model?" onConfirm={() => deleteModel(model.namespace, model.name).then(() => { message.success(t("common.success")); fetchData(); }).catch((err) => message.error(err instanceof Error ? err.message : String(err)))}>
                        <Button size="small" danger icon={<DeleteOutlined />} />
                      </Popconfirm>
                    </Space>
                  </div>
                </Space>
              </Card>
            </Col>
          ))}
        </Row>
      )}

      {/* Register Model Modal */}
      <Modal
        title="Register Model"
        open={registerOpen}
        onOk={handleRegister}
        onCancel={() => { setRegisterOpen(false); form.resetFields(); }}
        okText="Register"
        width={600}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label="Model Name" rules={[{ required: true, pattern: /^[a-z][a-z0-9-]*$/ }]}>
                <Input placeholder="qwen3-8b" />
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
          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="version" label="Version" initialValue="1.0">
                <Input placeholder="1.0" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="framework" label="Framework" initialValue="pytorch" rules={[{ required: true }]}>
                <Select options={[
                  { label: 'PyTorch (safetensors)', value: 'pytorch' },
                  { label: 'TensorFlow (SavedModel)', value: 'tensorflow' },
                  { label: 'ONNX', value: 'onnx' },
                  { label: 'Custom', value: 'custom' },
                ]} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="size" label="Model Size">
                <Select allowClear placeholder="e.g. 7B" options={[
                  { label: '0.5B', value: '0.5B' },
                  { label: '1.5B', value: '1.5B' },
                  { label: '3B', value: '3B' },
                  { label: '7B', value: '7B' },
                  { label: '14B', value: '14B' },
                  { label: '32B', value: '32B' },
                  { label: '72B', value: '72B' },
                  { label: '235B', value: '235B' },
                ]} />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="path" label="Model Path" rules={[{ required: true }]} tooltip="PVC mount path or OSS/S3 URI where model weights are stored">
            <Input placeholder="/models/Qwen3-8B or oss://bucket/models/qwen3-8b/" />
          </Form.Item>
          <Form.Item name="source" label="Source" initialValue="custom">
            <Select options={[
              { label: 'HuggingFace Hub', value: 'huggingface' },
              { label: 'ModelScope', value: 'modelscope' },
              { label: 'Training Output', value: 'training' },
              { label: 'Custom Upload', value: 'custom' },
            ]} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Models;
