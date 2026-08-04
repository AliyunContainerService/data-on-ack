import React, { useEffect, useState } from 'react';
import {
  Card, Button, Typography, Row, Col, Steps, Form, Select, Input, InputNumber,
  Space, Tag, message, Alert, Divider, Radio,
} from 'antd';
import {
  RocketOutlined, CheckCircleOutlined, ExperimentOutlined,
  DatabaseOutlined, SettingOutlined, SendOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { createTrainingJob, TrainingJobSpec } from '../api/training';
import { get } from '../api/client';

const { Title, Text, Paragraph } = Typography;

// Model options for fine-tuning
const BASE_MODELS = [
  { id: 'qwen3-8b', name: 'Qwen3-8B', size: '8B', family: 'Qwen', image: 'registry-cn-hangzhou.ack.aliyuncs.com/dev/ubuntu:24.04-update' },
  { id: 'qwen3-14b', name: 'Qwen3-14B', size: '14B', family: 'Qwen', image: 'registry-cn-hangzhou.ack.aliyuncs.com/dev/ubuntu:24.04-update' },
  { id: 'qwen3-32b', name: 'Qwen3-32B', size: '32B', family: 'Qwen', image: 'registry-cn-hangzhou.ack.aliyuncs.com/dev/ubuntu:24.04-update' },
  { id: 'llama3-8b', name: 'LLaMA-3-8B', size: '8B', family: 'LLaMA', image: 'registry-cn-hangzhou.ack.aliyuncs.com/dev/ubuntu:24.04-update' },
  { id: 'llama3-70b', name: 'LLaMA-3-70B', size: '70B', family: 'LLaMA', image: 'registry-cn-hangzhou.ack.aliyuncs.com/dev/ubuntu:24.04-update' },
  { id: 'deepseek-v3', name: 'DeepSeek-V3-Lite', size: '16B', family: 'DeepSeek', image: 'registry-cn-hangzhou.ack.aliyuncs.com/dev/ubuntu:24.04-update' },
  { id: 'yi-34b', name: 'Yi-1.5-34B', size: '34B', family: 'Yi', image: 'registry-cn-hangzhou.ack.aliyuncs.com/dev/ubuntu:24.04-update' },
  { id: 'mistral-7b', name: 'Mistral-7B-v0.3', size: '7B', family: 'Mistral', image: 'registry-cn-hangzhou.ack.aliyuncs.com/dev/ubuntu:24.04-update' },
];

// Fine-tuning methods with default hyperparameters
const METHODS = [
  {
    id: 'lora', name: 'LoRA', desc: '低秩适配，仅训练 0.1% 参数，单卡可跑 8B 模型',
    descEn: 'Low-Rank Adaptation, trains ~0.1% params, single GPU for 8B models',
    defaults: { lr: '2e-4', epochs: 3, batchSize: 8, gpuPerNode: 1, nodes: 1, loraRank: 64, loraAlpha: 128 },
    gpu: '1-2',
  },
  {
    id: 'qlora', name: 'QLoRA (4-bit)', desc: '4-bit 量化 + LoRA，显存减半，单卡可跑 14B 模型',
    descEn: '4-bit quantization + LoRA, halves VRAM, single GPU for 14B models',
    defaults: { lr: '1e-4', epochs: 3, batchSize: 4, gpuPerNode: 1, nodes: 1, loraRank: 64, loraAlpha: 128 },
    gpu: '1',
  },
  {
    id: 'full', name: '全参数微调', desc: '训练所有参数，效果最好但需要多机多卡 + DeepSpeed ZeRO-3',
    descEn: 'Full parameter fine-tuning, best quality, requires multi-node + DeepSpeed ZeRO-3',
    defaults: { lr: '5e-5', epochs: 2, batchSize: 2, gpuPerNode: 8, nodes: 4, loraRank: 0, loraAlpha: 0 },
    gpu: '8-32',
  },
  {
    id: 'dpo', name: 'DPO 对齐', desc: '直接偏好优化，需要偏好对数据集（chosen/rejected）',
    descEn: 'Direct Preference Optimization, requires preference pair dataset',
    defaults: { lr: '5e-7', epochs: 1, batchSize: 4, gpuPerNode: 2, nodes: 1, loraRank: 64, loraAlpha: 128 },
    gpu: '2-4',
  },
];

interface DatasetInfo {
  name: string;
  namespace: string;
  capacity: string;
}

const FineTune: React.FC = () => {
  const { i18n } = useTranslation();
  const isZh = i18n.language === 'zh';
  const [step, setStep] = useState(0);
  const [selectedModel, setSelectedModel] = useState<string>('');
  const [selectedDataset, setSelectedDataset] = useState<string>('');
  const [selectedMethod, setSelectedMethod] = useState<string>('lora');
  const [datasets, setDatasets] = useState<DatasetInfo[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [form] = Form.useForm();

  useEffect(() => {
    get<DatasetInfo[]>('/datasets').then((data) => setDatasets(data || [])).catch(() => {});
  }, []);

  const currentMethod = METHODS.find(m => m.id === selectedMethod);
  const currentModel = BASE_MODELS.find(m => m.id === selectedModel);

  const handleMethodChange = (methodId: string) => {
    setSelectedMethod(methodId);
    const method = METHODS.find(m => m.id === methodId);
    if (method) {
      form.setFieldsValue({
        lr: method.defaults.lr,
        epochs: method.defaults.epochs,
        batchSize: method.defaults.batchSize,
        gpuPerNode: method.defaults.gpuPerNode,
        nodes: method.defaults.nodes,
      });
    }
  };

  const handleSubmit = async () => {
    if (!currentModel || !currentMethod) return;
    try {
      const values = await form.validateFields();
      setSubmitting(true);

      const modelPath = `/models/${currentModel.name}`;
      const dataPath = selectedDataset ? `/data` : '/data/default';

      // Build training command based on method
      let command = '';
      switch (selectedMethod) {
        case 'lora':
        case 'qlora':
          command = `torchrun --nproc_per_node=${values.gpuPerNode} -m swift sft --model_type ${currentModel.id} --model_id_or_path ${modelPath} --dataset ${dataPath}/train.jsonl --output_dir /output/${values.jobName} --lora_rank ${values.loraRank || 64} --learning_rate ${values.lr} --num_train_epochs ${values.epochs} --per_device_train_batch_size ${values.batchSize}${selectedMethod === 'qlora' ? ' --quantization_bit 4' : ''}`;
          break;
        case 'full':
          command = `torchrun --nproc_per_node=${values.gpuPerNode} --nnodes=${values.nodes} --node_rank=$RANK --master_addr=$MASTER_ADDR --master_port=29500 train.py --model_name_or_path ${modelPath} --data_path ${dataPath}/train.jsonl --output_dir /output/${values.jobName} --learning_rate ${values.lr} --num_train_epochs ${values.epochs} --per_device_train_batch_size ${values.batchSize} --deepspeed ds_config_zero3.json`;
          break;
        case 'dpo':
          command = `torchrun --nproc_per_node=${values.gpuPerNode} -m swift rlhf --rlhf_type dpo --model_type ${currentModel.id} --model_id_or_path ${modelPath} --dataset ${dataPath}/dpo_pairs.jsonl --output_dir /output/${values.jobName} --learning_rate ${values.lr} --num_train_epochs ${values.epochs} --per_device_train_batch_size ${values.batchSize} --lora_rank ${values.loraRank || 64}`;
          break;
      }

      const spec: TrainingJobSpec = {
        name: values.jobName,
        namespace: values.namespace || 'default-group',
        kind: 'PyTorchJob',
        image: currentModel.image,
        command,
        workerCount: values.nodes || 1,
        workerCpu: values.cpuPerNode || '8',
        workerMemory: values.memPerNode || '32Gi',
        workerGpu: values.gpuPerNode || 1,
        env: {
          MODEL_NAME: currentModel.name,
          FINETUNE_METHOD: selectedMethod,
          NCCL_DEBUG: 'WARN',
        },
        dataSources: selectedDataset ? [{ name: selectedDataset, mountPath: '/data' }] : [],
      };

      await createTrainingJob(spec);
      message.success(isZh ? '微调任务已提交' : 'Fine-tuning job submitted');
      setStep(4); // success step
    } catch (err) {
      if (err instanceof Error) message.error(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  const steps = [
    { title: isZh ? '选择模型' : 'Select Model', icon: <ExperimentOutlined /> },
    { title: isZh ? '选择数据' : 'Select Data', icon: <DatabaseOutlined /> },
    { title: isZh ? '微调方法' : 'Method', icon: <SettingOutlined /> },
    { title: isZh ? '配置提交' : 'Configure', icon: <SendOutlined /> },
  ];

  return (
    <div>
      <div style={{ marginBottom: 24 }}>
        <Title level={4} style={{ marginBottom: 4 }}>{isZh ? '模型微调' : 'Fine-Tuning'}</Title>
        <Text type="secondary">{isZh ? '选择基座模型、数据集和微调方法，一键提交训练任务' : 'Select base model, dataset, and method to submit a fine-tuning job'}</Text>
      </div>

      <Card bordered={false} style={{ borderRadius: 14, marginBottom: 20 }}>
        <Steps current={step} size="small" items={steps} style={{ marginBottom: 32 }} />

        {/* Step 0: Select Model */}
        {step === 0 && (
          <div>
            <Text strong style={{ display: 'block', marginBottom: 12 }}>{isZh ? '选择基座模型' : 'Select Base Model'}</Text>
            <Row gutter={[12, 12]}>
              {BASE_MODELS.map((model) => (
                <Col xs={24} sm={12} md={8} lg={6} key={model.id}>
                  <Card
                    hoverable
                    size="small"
                    style={{ borderRadius: 10, border: selectedModel === model.id ? '2px solid #0071e3' : '1px solid #e8e8ed', cursor: 'pointer' }}
                    onClick={() => setSelectedModel(model.id)}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <Text strong style={{ fontSize: 13 }}>{model.name}</Text>
                      {selectedModel === model.id && <CheckCircleOutlined style={{ color: '#0071e3' }} />}
                    </div>
                    <Space size={4} style={{ marginTop: 6 }}>
                      <Tag color="blue">{model.size}</Tag>
                      <Tag>{model.family}</Tag>
                    </Space>
                  </Card>
                </Col>
              ))}
            </Row>
            <div style={{ marginTop: 20, textAlign: 'right' }}>
              <Button type="primary" disabled={!selectedModel} onClick={() => setStep(1)}>
                {isZh ? '下一步' : 'Next'}
              </Button>
            </div>
          </div>
        )}

        {/* Step 1: Select Dataset */}
        {step === 1 && (
          <div>
            <Text strong style={{ display: 'block', marginBottom: 12 }}>{isZh ? '选择训练数据集 (PVC)' : 'Select Training Dataset (PVC)'}</Text>
            {datasets.length === 0 ? (
              <Alert type="info" showIcon message={isZh ? '暂无可用数据集，请先到数据集页面创建 PVC' : 'No datasets available. Create a PVC in the Datasets page first.'} style={{ marginBottom: 16, borderRadius: 8 }} />
            ) : (
              <Radio.Group value={selectedDataset} onChange={(e) => setSelectedDataset(e.target.value)} style={{ width: '100%' }}>
                <Row gutter={[12, 12]}>
                  {datasets.filter(d => d.namespace === 'default-group' || d.namespace === 'default').map((ds) => (
                    <Col xs={24} sm={12} md={8} key={ds.name}>
                      <Card size="small" hoverable style={{ borderRadius: 10, border: selectedDataset === ds.name ? '2px solid #0071e3' : undefined }}>
                        <Radio value={ds.name}>
                          <Text strong>{ds.name}</Text>
                          <div><Text type="secondary" style={{ fontSize: 11 }}>{ds.namespace} | {ds.capacity}</Text></div>
                        </Radio>
                      </Card>
                    </Col>
                  ))}
                </Row>
              </Radio.Group>
            )}
            <div style={{ marginTop: 20, display: 'flex', justifyContent: 'space-between' }}>
              <Button onClick={() => setStep(0)}>{isZh ? '上一步' : 'Back'}</Button>
              <Button type="primary" onClick={() => setStep(2)}>
                {selectedDataset ? (isZh ? '下一步' : 'Next') : (isZh ? '跳过，稍后指定' : 'Skip for now')}
              </Button>
            </div>
          </div>
        )}

        {/* Step 2: Select Method */}
        {step === 2 && (
          <div>
            <Text strong style={{ display: 'block', marginBottom: 12 }}>{isZh ? '选择微调方法' : 'Select Fine-Tuning Method'}</Text>
            <Row gutter={[12, 12]}>
              {METHODS.map((method) => (
                <Col xs={24} sm={12} key={method.id}>
                  <Card
                    hoverable
                    size="small"
                    style={{ borderRadius: 10, border: selectedMethod === method.id ? '2px solid #0071e3' : '1px solid #e8e8ed', cursor: 'pointer' }}
                    onClick={() => handleMethodChange(method.id)}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <Text strong>{method.name}</Text>
                      <Tag color="volcano">{method.gpu} GPU</Tag>
                    </div>
                    <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 4 }}>
                      {isZh ? method.desc : method.descEn}
                    </Text>
                  </Card>
                </Col>
              ))}
            </Row>
            <div style={{ marginTop: 20, display: 'flex', justifyContent: 'space-between' }}>
              <Button onClick={() => setStep(1)}>{isZh ? '上一步' : 'Back'}</Button>
              <Button type="primary" onClick={() => { setStep(3); handleMethodChange(selectedMethod); }}>
                {isZh ? '下一步' : 'Next'}
              </Button>
            </div>
          </div>
        )}

        {/* Step 3: Configure & Submit */}
        {step === 3 && (
          <div>
            {/* Summary bar */}
            <div style={{ background: '#f0f5ff', borderRadius: 10, padding: '12px 16px', marginBottom: 20 }}>
              <Space split={<Divider type="vertical" />}>
                <span><Text type="secondary">{isZh ? '模型' : 'Model'}:</Text> <Text strong>{currentModel?.name}</Text></span>
                <span><Text type="secondary">{isZh ? '数据' : 'Data'}:</Text> <Text strong>{selectedDataset || (isZh ? '未指定' : 'N/A')}</Text></span>
                <span><Text type="secondary">{isZh ? '方法' : 'Method'}:</Text> <Tag color="blue">{currentMethod?.name}</Tag></span>
              </Space>
            </div>

            <Form form={form} layout="vertical" initialValues={{
              jobName: `${currentModel?.id || 'model'}-${selectedMethod}-${Date.now().toString(36).slice(-4)}`,
              namespace: 'default-group',
              lr: currentMethod?.defaults.lr,
              epochs: currentMethod?.defaults.epochs,
              batchSize: currentMethod?.defaults.batchSize,
              gpuPerNode: currentMethod?.defaults.gpuPerNode,
              nodes: currentMethod?.defaults.nodes,
              cpuPerNode: '8',
              memPerNode: '32Gi',
              loraRank: currentMethod?.defaults.loraRank || 64,
            }}>
              <Row gutter={16}>
                <Col span={12}>
                  <Form.Item name="jobName" label={isZh ? '任务名称' : 'Job Name'} rules={[{ required: true, pattern: /^[a-z][a-z0-9-]*$/ }]}>
                    <Input />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item name="namespace" label={isZh ? '命名空间' : 'Namespace'}>
                    <Select options={[
                      { label: 'default-group', value: 'default-group' },
                      { label: 'default', value: 'default' },
                    ]} />
                  </Form.Item>
                </Col>
              </Row>

              <Divider style={{ margin: '12px 0' }}>{isZh ? '超参数' : 'Hyperparameters'}</Divider>
              <Row gutter={16}>
                <Col span={6}>
                  <Form.Item name="lr" label={isZh ? '学习率' : 'Learning Rate'}>
                    <Input />
                  </Form.Item>
                </Col>
                <Col span={6}>
                  <Form.Item name="epochs" label={isZh ? '训练轮数' : 'Epochs'}>
                    <InputNumber min={1} max={100} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col span={6}>
                  <Form.Item name="batchSize" label={isZh ? '批大小' : 'Batch Size'}>
                    <InputNumber min={1} max={256} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                {(selectedMethod === 'lora' || selectedMethod === 'qlora' || selectedMethod === 'dpo') && (
                  <Col span={6}>
                    <Form.Item name="loraRank" label="LoRA Rank">
                      <Select options={[
                        { label: '8', value: 8 },
                        { label: '16', value: 16 },
                        { label: '32', value: 32 },
                        { label: '64', value: 64 },
                        { label: '128', value: 128 },
                      ]} />
                    </Form.Item>
                  </Col>
                )}
              </Row>

              <Divider style={{ margin: '12px 0' }}>{isZh ? '计算资源' : 'Resources'}</Divider>
              <Row gutter={16}>
                <Col span={6}>
                  <Form.Item name="nodes" label={isZh ? '节点数' : 'Nodes'}>
                    <InputNumber min={1} max={32} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col span={6}>
                  <Form.Item name="gpuPerNode" label={isZh ? 'GPU / 节点' : 'GPU / Node'}>
                    <InputNumber min={0} max={8} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col span={6}>
                  <Form.Item name="cpuPerNode" label={isZh ? 'CPU / 节点' : 'CPU / Node'}>
                    <Input addonAfter="cores" />
                  </Form.Item>
                </Col>
                <Col span={6}>
                  <Form.Item name="memPerNode" label={isZh ? '内存 / 节点' : 'Memory / Node'}>
                    <Select options={[
                      { label: '16 GiB', value: '16Gi' },
                      { label: '32 GiB', value: '32Gi' },
                      { label: '64 GiB', value: '64Gi' },
                      { label: '128 GiB', value: '128Gi' },
                    ]} />
                  </Form.Item>
                </Col>
              </Row>
            </Form>

            <div style={{ marginTop: 20, display: 'flex', justifyContent: 'space-between' }}>
              <Button onClick={() => setStep(2)}>{isZh ? '上一步' : 'Back'}</Button>
              <Button type="primary" icon={<RocketOutlined />} loading={submitting} onClick={handleSubmit}>
                {submitting ? (isZh ? '提交中...' : 'Submitting...') : (isZh ? '提交微调任务' : 'Submit Fine-Tune Job')}
              </Button>
            </div>
          </div>
        )}

        {/* Step 4: Success */}
        {step === 4 && (
          <div style={{ textAlign: 'center', padding: '40px 0' }}>
            <CheckCircleOutlined style={{ fontSize: 48, color: '#52c41a', marginBottom: 16 }} />
            <Title level={4}>{isZh ? '微调任务已提交' : 'Fine-Tuning Job Submitted'}</Title>
            <Paragraph type="secondary">
              {isZh ? '任务已提交到集群，您可以在训练页面查看进度和日志。' : 'The job has been submitted. Check progress and logs on the Training page.'}
            </Paragraph>
            <Space>
              <Button type="primary" onClick={() => window.location.href = '/training'}>{isZh ? '查看训练任务' : 'View Training Jobs'}</Button>
              <Button onClick={() => { setStep(0); setSelectedModel(''); setSelectedDataset(''); form.resetFields(); }}>{isZh ? '创建新任务' : 'Create Another'}</Button>
            </Space>
          </div>
        )}
      </Card>
    </div>
  );
};

export default FineTune;
