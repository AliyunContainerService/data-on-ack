import React, { useEffect, useState } from 'react';
import {
  Card, Table, Button, Tag, Space, Modal, Form, Input, InputNumber,
  Select, message, Popconfirm, Typography, Row, Col, Badge, Segmented,
  Switch, Divider, Collapse, Drawer, Spin, Tabs, Tooltip, Alert,
} from 'antd';
import {
  PlusOutlined, DeleteOutlined, ReloadOutlined, FileTextOutlined,
  CopyOutlined, ThunderboltOutlined, MedicineBoxOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  listTrainingJobs, createTrainingJob, deleteTrainingJob,
  listTrainingJobPods, getTrainingJobLogs, getCapabilities,
  getTrainingJobEvents, getTrainingJobYAML, getTrainingJobMetrics,
  listExperiments, addExperimentRun,
  TrainingJobInfo, TrainingJobSpec, PodInfo, EventInfo,
  GPUMetricsResponse, ExperimentInfo,
} from '../api/training';
import { useAssistantStore } from '../store/assistant';

const { Title, Text } = Typography;

const statusConfig: Record<string, { status: 'processing' | 'success' | 'error' | 'warning' | 'default'; text: string }> = {
  Running: { status: 'processing', text: 'Running' },
  Succeeded: { status: 'success', text: 'Succeeded' },
  Failed: { status: 'error', text: 'Failed' },
  Pending: { status: 'warning', text: 'Pending' },
};

const Training: React.FC = () => {
  const { t } = useTranslation();
  const assistantAvailable = useAssistantStore((st) => st.available);
  const requestDiagnose = useAssistantStore((st) => st.requestDiagnose);
  const [jobs, setJobs] = useState<TrainingJobInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [form] = Form.useForm();
  const [trainingMode, setTrainingMode] = useState<string>('single');
  const [selectedKind, setSelectedKind] = useState<string>('PyTorchJob');

  // Cluster capabilities
  const [raySupport, setRaySupport] = useState(false);

  // Experiments list for linking
  const [experiments, setExperiments] = useState<ExperimentInfo[]>([]);

  // Detail drawer state
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailJob, setDetailJob] = useState<TrainingJobInfo | null>(null);
  const [detailPods, setDetailPods] = useState<PodInfo[]>([]);
  const [detailEvents, setDetailEvents] = useState<EventInfo[]>([]);
  const [detailYAML, setDetailYAML] = useState<string>('');
  const [detailTab, setDetailTab] = useState<string>('pods');

  // GPU Metrics state
  const [metricsData, setMetricsData] = useState<GPUMetricsResponse | null>(null);
  const [metricsLoading, setMetricsLoading] = useState(false);
  const [selectedMetric, setSelectedMetric] = useState<string>('gpu_util');

  // Log viewer state
  const [logDrawerOpen, setLogDrawerOpen] = useState(false);
  const [logJob, setLogJob] = useState<TrainingJobInfo | null>(null);
  const [pods, setPods] = useState<PodInfo[]>([]);
  const [selectedPod, setSelectedPod] = useState<string>('');
  const [logContent, setLogContent] = useState<string>('');
  const [logLoading, setLogLoading] = useState(false);
  const [podsLoading, setPodsLoading] = useState(false);
  const [logTailLines, setLogTailLines] = useState<number>(500);
  const [logSearch, setLogSearch] = useState<string>('');
  const [autoRefresh, setAutoRefresh] = useState(false);
  const autoRefreshRef = React.useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchData = () => {
    setLoading(true);
    listTrainingJobs()
      .then((data) => setJobs(data || []))
      .catch(() => setJobs([]))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchData();
    getCapabilities().then((caps) => setRaySupport(caps.raySupport)).catch(() => {});
    listExperiments().then((exps) => setExperiments(exps || [])).catch(() => {});
  }, []);

  const handleViewLogs = async (record: TrainingJobInfo) => {
    setLogJob(record);
    setLogDrawerOpen(true);
    setLogContent('');
    setSelectedPod('');
    setPodsLoading(true);
    try {
      const podList = await listTrainingJobPods(record.namespace, record.name, record.kind);
      setPods(podList || []);
      // Auto-select first pod
      if (podList && podList.length > 0) {
        setSelectedPod(podList[0].name);
        fetchLogs(record.namespace, record.name, podList[0].name);
      }
    } catch {
      setPods([]);
    } finally {
      setPodsLoading(false);
    }
  };

  const fetchLogs = async (namespace: string, jobName: string, podName: string) => {
    setLogLoading(true);
    try {
      const logs = await getTrainingJobLogs(namespace, jobName, podName, 1000);
      setLogContent(logs || '(no logs available)');
    } catch (err) {
      setLogContent(err instanceof Error ? `Error: ${err.message}` : 'Failed to fetch logs');
    } finally {
      setLogLoading(false);
    }
  };

  const handlePodChange = (podName: string) => {
    setSelectedPod(podName);
    if (logJob) {
      fetchLogs(logJob.namespace, logJob.name, podName);
    }
  };

  // Auto-refresh logs
  React.useEffect(() => {
    if (autoRefresh && logJob && selectedPod) {
      autoRefreshRef.current = setInterval(() => {
        fetchLogs(logJob.namespace, logJob.name, selectedPod);
      }, 5000);
    }
    return () => {
      if (autoRefreshRef.current) {
        clearInterval(autoRefreshRef.current);
        autoRefreshRef.current = null;
      }
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoRefresh, logJob, selectedPod]);

  const fetchMetrics = async (job: TrainingJobInfo, metric: string) => {
    setMetricsLoading(true);
    try {
      const data = await getTrainingJobMetrics(job.namespace, job.name, metric, job.kind);
      setMetricsData(data);
    } catch {
      setMetricsData({ available: false, message: 'Failed to fetch metrics' });
    } finally {
      setMetricsLoading(false);
    }
  };

  const handleViewDetail = async (record: TrainingJobInfo) => {
    setDetailJob(record);
    setDetailOpen(true);
    setDetailTab('pods');
    setMetricsData(null);
    // Fetch all detail data in parallel
    Promise.all([
      listTrainingJobPods(record.namespace, record.name, record.kind).catch(() => []),
      getTrainingJobEvents(record.namespace, record.name).catch(() => []),
      getTrainingJobYAML(record.namespace, record.name, record.kind).catch(() => ({})),
    ]).then(([pods, events, yaml]) => {
      setDetailPods(pods || []);
      setDetailEvents(events || []);
      setDetailYAML(JSON.stringify(yaml, null, 2));
    });
  };

  const handleCreate = async () => {
    try {
      const values = await form.validateFields();
      const spec: TrainingJobSpec = {
        name: values.name,
        namespace: values.namespace || 'default',
        kind: values.kind,
        image: values.image,
        command: values.command,
        workerCount: trainingMode === 'single' ? 1 : (values.workerCount || 1),
        workerCpu: values.workerCpu || '4',
        workerMemory: values.workerMemory || '16Gi',
        workerGpu: values.workerGpu || 0,
        env: {},
        dataSources: [],
      };

      // Include script if provided
      if (values.script) {
        spec.script = values.script;
      }

      // Include shm size
      if (values.shmSize) {
        spec.shmSize = values.shmSize;
      }

      // RDMA and host network
      if (values.enableRdma) spec.enableRdma = true;
      if (values.hostNetwork) spec.hostNetwork = true;
      if (values.queue) spec.queue = values.queue;
      if (values.priority) spec.priority = values.priority;

      // Parse environment variables
      if (values.envVars) {
        const envObj: Record<string, string> = {};
        (values.envVars as string).split('\n').forEach((line: string) => {
          const idx = line.indexOf('=');
          if (idx > 0) {
            envObj[line.slice(0, idx).trim()] = line.slice(idx + 1).trim();
          }
        });
        spec.env = envObj;
      }

      // Parse data mounts
      if (values.dataPvc && values.dataMountPath) {
        spec.dataSources = [{ name: values.dataPvc, mountPath: values.dataMountPath }];
      }

      setSubmitting(true);
      await createTrainingJob(spec);

      // Link to experiment if selected
      if (values.experiment) {
        try {
          await addExperimentRun(values.experiment, {
            jobName: spec.name,
            jobKind: spec.kind,
            params: spec.env || {},
            status: 'Pending',
          }, spec.namespace);
        } catch { /* non-blocking */ }
      }

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

  const handleRerun = async (record: TrainingJobInfo) => {
    // Fetch raw YAML and parse all fields to pre-fill the create form
    try {
      const raw = await getTrainingJobYAML(record.namespace, record.name, record.kind);
      const spec = (raw as Record<string, unknown>).spec as Record<string, unknown> | undefined;
      
      // Extract container info from the CRD spec
      let image = '', command = '', envStr = '', workerCpu = '4', workerMemory = '16Gi', workerGpu = record.gpu;
      let workerCount = 1;
      let dataPvc = '', dataMountPath = '';
      
      // Navigate replica specs to find the container
      const replicaKeys = ['pytorchReplicaSpecs', 'tfReplicaSpecs', 'mpiReplicaSpecs'];
      for (const key of replicaKeys) {
        const replicaSpecs = spec?.[key] as Record<string, unknown> | undefined;
        if (!replicaSpecs) continue;
        const worker = (replicaSpecs.Worker || replicaSpecs.worker) as Record<string, unknown> | undefined;
        if (!worker) continue;
        if (worker.replicas) workerCount = worker.replicas as number;
        const template = worker.template as Record<string, unknown> | undefined;
        const podSpec = template?.spec as Record<string, unknown> | undefined;
        const containers = podSpec?.containers as Array<Record<string, unknown>> | undefined;
        if (containers && containers.length > 0) {
          const c = containers[0];
          image = (c.image as string) || '';
          const cmd = c.command as string[] | undefined;
          if (cmd && cmd.length >= 3) command = cmd[2] as string; // ["sh", "-c", "actual command"]
          // Parse resources
          const res = c.resources as Record<string, unknown> | undefined;
          const limits = res?.limits as Record<string, string> | undefined;
          if (limits) {
            workerCpu = limits.cpu || '4';
            workerMemory = limits.memory || '16Gi';
            if (limits['nvidia.com/gpu']) workerGpu = parseInt(limits['nvidia.com/gpu']) || 0;
          }
          // Parse env vars
          const envArr = c.env as Array<{ name: string; value: string }> | undefined;
          if (envArr) {
            envStr = envArr.map(e => `${e.name}=${e.value}`).join('\n');
          }
          // Parse volume mounts for data
          const vms = c.volumeMounts as Array<{ name: string; mountPath: string }> | undefined;
          const vols = podSpec?.volumes as Array<{ name: string; persistentVolumeClaim?: { claimName: string } }> | undefined;
          if (vms && vols) {
            for (const vm of vms) {
              const vol = vols.find(v => v.name === vm.name);
              if (vol?.persistentVolumeClaim) {
                dataPvc = vol.persistentVolumeClaim.claimName;
                dataMountPath = vm.mountPath;
                break;
              }
            }
          }
        }
        break;
      }

      setSelectedKind(record.kind);
      setTrainingMode(workerCount > 1 ? 'distributed' : 'single');
      form.setFieldsValue({
        name: record.name + '-rerun',
        namespace: record.namespace,
        kind: record.kind,
        image,
        command,
        workerCount,
        workerCpu,
        workerMemory,
        workerGpu,
        envVars: envStr,
        dataPvc,
        dataMountPath,
      });
    } catch {
      // Fallback: minimal pre-fill
      form.setFieldsValue({
        name: record.name + '-rerun',
        namespace: record.namespace,
        kind: record.kind,
        workerGpu: record.gpu,
      });
    }
    setCreateOpen(true);
  };

  const columns = [
    {
      title: t('training.name'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: TrainingJobInfo) => (
        <Space direction="vertical" size={0}>
          <Text strong style={{ cursor: 'pointer', color: '#0071e3' }} onClick={() => handleViewDetail(record)}>{name}</Text>
          <Text type="secondary" style={{ fontSize: 11 }}>{record.namespace}</Text>
        </Space>
      ),
    },
    {
      title: t('training.kind'),
      dataIndex: 'kind',
      key: 'kind',
      width: 150,
      render: (kind: string, record: TrainingJobInfo) => (
        <Space size={4}>
          <Tag color={kind === 'RayJob' ? 'purple' : 'geekblue'}>{kind}</Tag>
          {kind === 'RayJob' && record.status === 'Running' && (
            <Button type="link" size="small" style={{ fontSize: 11, padding: 0 }} href={`/ray/${record.namespace}/${record.name}/`} target="_blank">
              Dashboard
            </Button>
          )}
        </Space>
      ),
    },
    {
      title: t('training.status'),
      dataIndex: 'status',
      key: 'status',
      width: 160,
      render: (status: string, record: TrainingJobInfo) => {
        const cfg = statusConfig[status] || { status: 'default' as const, text: status };
        const hasReason = record.reason || record.message;
        const badge = <Badge status={cfg.status} text={cfg.text} />;
        if (hasReason && (status === 'Pending' || status === 'Failed')) {
          return (
            <Tooltip title={<div><strong>{record.reason}</strong>{record.message && <div style={{ marginTop: 4 }}>{record.message}</div>}</div>} placement="topLeft">
              <Space size={4} style={{ cursor: 'help' }}>
                {badge}
                <span style={{ fontSize: 10, color: status === 'Failed' ? '#ff4d4f' : '#faad14', maxWidth: 80, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', display: 'inline-block' }}>
                  {record.reason}
                </span>
              </Space>
            </Tooltip>
          );
        }
        return badge;
      },
    },
    {
      title: t('training.duration'),
      dataIndex: 'duration',
      key: 'duration',
      width: 100,
    },
    {
      title: t('training.gpu'),
      dataIndex: 'gpu',
      key: 'gpu',
      width: 80,
      render: (gpu: number) => gpu > 0 ? <Tag color="volcano">{gpu} GPU</Tag> : <Tag>CPU</Tag>,
    },
    {
      title: t('training.actions'),
      key: 'actions',
      width: 280,
      render: (_: unknown, record: TrainingJobInfo) => (
        <Space size={4}>
          <Button size="small" icon={<FileTextOutlined />} onClick={() => handleViewLogs(record)}>{t('training.logs')}</Button>
          <Button size="small" icon={<CopyOutlined />} onClick={() => handleRerun(record)}>{t('training.rerun')}</Button>
          {assistantAvailable && (
            <Button size="small" icon={<MedicineBoxOutlined />} onClick={() => requestDiagnose({ namespace: record.namespace, name: record.name, kind: record.kind })}>{t('training.diagnose')}</Button>
          )}
          <Popconfirm title={t('common.delete.confirm')} onConfirm={() => deleteTrainingJob(record.namespace, record.name, record.kind).then(() => { message.success(t('common.success')); fetchData(); }).catch((err) => message.error(err instanceof Error ? err.message : String(err)))}>
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
          <Title level={4} style={{ marginBottom: 4 }}>{t('training.title')}</Title>
          <Text type="secondary">{t('training.desc')}</Text>
        </div>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchData} />
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            {t('training.create')}
          </Button>
        </Space>
      </div>

      <Card bordered={false} style={{ borderRadius: 14 }}>
        <Table columns={columns} dataSource={jobs} rowKey={(r) => `${r.namespace}/${r.name}`} loading={loading} pagination={false} size="middle" />
      </Card>

      {/* Log Viewer Drawer */}
      <Drawer
        title={<Space><FileTextOutlined />{t('training.logs.title')}: {logJob?.name}</Space>}
        open={logDrawerOpen}
        onClose={() => { setLogDrawerOpen(false); setLogJob(null); setPods([]); setLogContent(''); setAutoRefresh(false); setLogSearch(''); }}
        width={780}
        styles={{ body: { padding: 0 } }}
        extra={
          <Space size={8}>
            <Select size="small" value={logTailLines} onChange={(v) => { setLogTailLines(v); if (logJob && selectedPod) fetchLogs(logJob.namespace, logJob.name, selectedPod); }} style={{ width: 110 }} options={[
              { label: 'Tail 100', value: 100 },
              { label: 'Tail 500', value: 500 },
              { label: 'Tail 2000', value: 2000 },
              { label: 'All', value: 10000 },
            ]} />
            <Switch size="small" checked={autoRefresh} onChange={setAutoRefresh} checkedChildren="Auto" unCheckedChildren="Manual" />
            <Button size="small" icon={<ReloadOutlined />} onClick={() => { if (logJob && selectedPod) fetchLogs(logJob.namespace, logJob.name, selectedPod); }} />
            <Button size="small" onClick={() => { navigator.clipboard.writeText(logContent); message.success('Copied!'); }}>{t('training.logs.copy')}</Button>
          </Space>
        }
      >
        {podsLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
        ) : (
          <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
            {/* Pod selector tabs */}
            {pods.length > 0 && (
              <Tabs
                activeKey={selectedPod}
                onChange={handlePodChange}
                style={{ padding: '0 16px' }}
                items={pods.map((p) => ({
                  key: p.name,
                  label: (
                    <Space size={4}>
                      <Badge status={p.status === 'Running' ? 'processing' : p.status === 'Succeeded' ? 'success' : p.status === 'Failed' ? 'error' : 'default'} />
                      <span>{p.role}/{p.name.split('-').pop()}</span>
                    </Space>
                  ),
                }))}
              />
            )}
            {pods.length === 0 && !podsLoading && (
              <div style={{ padding: 16, color: '#999' }}>{t('training.logs.noPods')}</div>
            )}
            {/* Pod info bar + search */}
            {selectedPod && pods.length > 0 && (() => {
              const pod = pods.find(p => p.name === selectedPod);
              return pod ? (
                <div style={{ padding: '8px 16px', background: '#f9f9fb', borderBottom: '1px solid #f0f0f0', fontSize: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Space split={<Divider type="vertical" />}>
                    <span>Node: <strong>{pod.node || '-'}</strong></span>
                    <span>IP: {pod.ip || '-'}</span>
                    <span>Restarts: {pod.restarts}</span>
                    <span>Status: <Badge status={pod.status === 'Running' ? 'processing' : pod.status === 'Succeeded' ? 'success' : 'error'} text={pod.status} /></span>
                  </Space>
                  <Input
                    size="small"
                    placeholder={t('training.logs.search')}
                    value={logSearch}
                    onChange={(e) => setLogSearch(e.target.value)}
                    allowClear
                    style={{ width: 160 }}
                  />
                </div>
              ) : null;
            })()}
            {/* Log content */}
            <div style={{ flex: 1, overflow: 'auto', padding: 16 }}>
              {logLoading && !logContent ? (
                <div style={{ textAlign: 'center', padding: 40 }}><Spin tip="Loading logs..." /></div>
              ) : (
                <div style={{ position: 'relative' }}>
                  {logLoading && <div style={{ position: 'absolute', top: 8, right: 8 }}><Spin size="small" /></div>}
                  {autoRefresh && <div style={{ position: 'absolute', top: 8, left: 8 }}><Badge status="processing" text={<span style={{ fontSize: 10, color: '#52c41a' }}>Live</span>} /></div>}
                  <pre style={{
                    background: '#1e1e1e',
                    color: '#d4d4d4',
                    padding: 16,
                    paddingTop: autoRefresh ? 32 : 16,
                    borderRadius: 8,
                    fontSize: 12,
                    fontFamily: 'SF Mono, Monaco, Menlo, monospace',
                    lineHeight: 1.6,
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-all',
                    minHeight: 400,
                    margin: 0,
                    maxHeight: 'calc(100vh - 260px)',
                    overflow: 'auto',
                  }}>
                    {logSearch
                      ? logContent.split('\n').filter(line => line.toLowerCase().includes(logSearch.toLowerCase())).join('\n') || '(no matching lines)'
                      : logContent}
                  </pre>
                </div>
              )}
            </div>
          </div>
        )}
      </Drawer>

      {/* Job Detail Drawer */}
      <Drawer
        title={<Space><ThunderboltOutlined />Detail: {detailJob?.name} <Tag color={detailJob?.kind === 'RayJob' ? 'purple' : 'geekblue'}>{detailJob?.kind}</Tag></Space>}
        open={detailOpen}
        onClose={() => { setDetailOpen(false); setDetailJob(null); }}
        width={720}
        styles={{ body: { padding: 0 } }}
        extra={
          <Space>
            <Button size="small" icon={<FileTextOutlined />} onClick={() => { if (detailJob) handleViewLogs(detailJob); }}>Logs</Button>
            <Button size="small" icon={<CopyOutlined />} onClick={() => { if (detailJob) handleRerun(detailJob); }}>Re-run</Button>
            {assistantAvailable && (
              <Button size="small" icon={<MedicineBoxOutlined />} onClick={() => { if (detailJob) requestDiagnose({ namespace: detailJob.namespace, name: detailJob.name, kind: detailJob.kind }); }}>
                {t('training.diagnose')}
              </Button>
            )}
          </Space>
        }
      >
        {/* Job summary header */}
        {detailJob && (
          <div style={{ padding: '16px 20px', background: '#f9f9fb', borderBottom: '1px solid #f0f0f0' }}>
            <Row gutter={16}>
              <Col span={6}>
                <Text type="secondary" style={{ fontSize: 11, display: 'block' }}>Status</Text>
                <Badge status={statusConfig[detailJob.status]?.status || 'default'} text={<Text strong>{detailJob.status}</Text>} />
              </Col>
              <Col span={6}>
                <Text type="secondary" style={{ fontSize: 11, display: 'block' }}>Duration</Text>
                <Text strong>{detailJob.duration || '-'}</Text>
              </Col>
              <Col span={6}>
                <Text type="secondary" style={{ fontSize: 11, display: 'block' }}>GPU</Text>
                <Text strong>{detailJob.gpu > 0 ? `${detailJob.gpu} GPU` : 'CPU'}</Text>
              </Col>
              <Col span={6}>
                <Text type="secondary" style={{ fontSize: 11, display: 'block' }}>Created</Text>
                <Text strong style={{ fontSize: 12 }}>{detailJob.createTime ? new Date(detailJob.createTime).toLocaleString() : '-'}</Text>
              </Col>
            </Row>
            {/* Status reason alert */}
            {(detailJob.reason || detailJob.message) && (detailJob.status === 'Pending' || detailJob.status === 'Failed') && (
              <Alert
                type={detailJob.status === 'Failed' ? 'error' : 'warning'}
                showIcon
                style={{ marginTop: 12, borderRadius: 8 }}
                message={<Text strong style={{ fontSize: 12 }}>{detailJob.reason || detailJob.status}</Text>}
                description={<Text style={{ fontSize: 12 }}>{detailJob.message}</Text>}
              />
            )}
          </div>
        )}
        <Tabs activeKey={detailTab} onChange={setDetailTab} style={{ padding: '0 16px' }} items={[
          {
            key: 'pods',
            label: `Pods (${detailPods.length})`,
            children: (
              <div style={{ padding: '0 0 16px' }}>
                {detailPods.length === 0 ? (
                  <div style={{ padding: 24, textAlign: 'center', color: '#999' }}>No pods found</div>
                ) : (
                  <table style={{ width: '100%', fontSize: 12, borderCollapse: 'collapse' }}>
                    <thead>
                      <tr style={{ background: '#f9f9fb', textAlign: 'left' }}>
                        <th style={{ padding: '8px 12px' }}>Pod</th>
                        <th style={{ padding: '8px 12px' }}>Role</th>
                        <th style={{ padding: '8px 12px' }}>Status</th>
                        <th style={{ padding: '8px 12px' }}>Node</th>
                        <th style={{ padding: '8px 12px' }}>IP</th>
                        <th style={{ padding: '8px 12px' }}>Restarts</th>
                        <th style={{ padding: '8px 12px' }}></th>
                      </tr>
                    </thead>
                    <tbody>
                      {detailPods.map((pod) => (
                        <tr key={pod.name} style={{ borderBottom: '1px solid #f0f0f0' }}>
                          <td style={{ padding: '8px 12px', fontFamily: 'monospace', fontSize: 11 }}>{pod.name}</td>
                          <td style={{ padding: '8px 12px' }}><Tag>{pod.role}</Tag></td>
                          <td style={{ padding: '8px 12px' }}>
                            <Badge status={pod.status === 'Running' ? 'processing' : pod.status === 'Succeeded' ? 'success' : pod.status === 'Failed' ? 'error' : 'warning'} text={pod.status} />
                          </td>
                          <td style={{ padding: '8px 12px', fontSize: 11 }}>{pod.node || '-'}</td>
                          <td style={{ padding: '8px 12px', fontSize: 11 }}>{pod.ip || '-'}</td>
                          <td style={{ padding: '8px 12px' }}>{pod.restarts}</td>
                          <td style={{ padding: '8px 12px' }}>
                            <Button type="link" size="small" icon={<FileTextOutlined />} onClick={() => {
                              if (detailJob) {
                                setDetailOpen(false);
                                setLogJob(detailJob);
                                setLogDrawerOpen(true);
                                setPods(detailPods);
                                setSelectedPod(pod.name);
                                fetchLogs(detailJob.namespace, detailJob.name, pod.name);
                              }
                            }}>Logs</Button>
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
            key: 'events',
            label: `Events (${detailEvents.length})`,
            children: (
              <div style={{ padding: '0 0 16px', maxHeight: 400, overflow: 'auto' }}>
                {detailEvents.length === 0 ? (
                  <div style={{ padding: 24, textAlign: 'center', color: '#999' }}>No events</div>
                ) : (
                  detailEvents.map((ev, idx) => (
                    <div key={idx} style={{ padding: '8px 12px', borderBottom: '1px solid #f5f5f5', fontSize: 12 }}>
                      <Space size={8}>
                        <Tag color={ev.type === 'Warning' ? 'orange' : 'default'} style={{ fontSize: 10 }}>{ev.type}</Tag>
                        <Text strong>{ev.reason}</Text>
                        <Text type="secondary">{ev.object}</Text>
                        {ev.count > 1 && <Tag style={{ fontSize: 10 }}>x{ev.count}</Tag>}
                      </Space>
                      <div style={{ marginTop: 4, color: '#555' }}>{ev.message}</div>
                      <Text type="secondary" style={{ fontSize: 10 }}>{ev.time}</Text>
                    </div>
                  ))
                )}
              </div>
            ),
          },
          {
            key: 'metrics',
            label: t('training.detail.metrics'),
            children: (
              <div style={{ padding: '12px 0 16px' }}>
                <div style={{ marginBottom: 12, display: 'flex', alignItems: 'center', gap: 12 }}>
                  <Select
                    size="small"
                    value={selectedMetric}
                    onChange={(v) => { setSelectedMetric(v); if (detailJob) fetchMetrics(detailJob, v); }}
                    style={{ width: 180 }}
                    options={[
                      { label: 'GPU Utilization', value: 'gpu_util' },
                      { label: 'Memory Used (FB)', value: 'fb_used' },
                      { label: 'SM Occupancy', value: 'sm_occupancy' },
                      { label: 'Power Usage', value: 'power_usage' },
                      { label: 'Temperature', value: 'gpu_temp' },
                    ]}
                  />
                  <Button size="small" icon={<ReloadOutlined />} onClick={() => { if (detailJob) fetchMetrics(detailJob, selectedMetric); }}>
                    {t('common.refresh')}
                  </Button>
                </div>
                {metricsLoading ? (
                  <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
                ) : metricsData ? (
                  metricsData.available ? (
                    <div>
                      {metricsData.pods && metricsData.pods.length > 0 ? (
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                          {metricsData.pods.map((pm) => (
                            <div key={pm.pod} style={{ background: '#f9f9fb', borderRadius: 10, padding: 14 }}>
                              <div style={{ fontSize: 12, marginBottom: 8, display: 'flex', justifyContent: 'space-between' }}>
                                <Text strong style={{ fontFamily: 'monospace', fontSize: 11 }}>{pm.pod}</Text>
                                <Space size={8}>
                                  <Text type="secondary" style={{ fontSize: 11 }}>Node: {pm.node}</Text>
                                  <Tag style={{ fontSize: 10 }}>{pm.gpu} GPU</Tag>
                                </Space>
                              </div>
                              {/* Sparkline bar visualization */}
                              <div style={{ display: 'flex', gap: 4, alignItems: 'flex-end', height: 40 }}>
                                {pm.metrics.map((m, idx) => {
                                  const maxVal = selectedMetric === 'fb_used' ? 81920 : selectedMetric === 'gpu_temp' ? 100 : selectedMetric === 'power_usage' ? 300 : 100;
                                  const pct = Math.min(100, (m.value / maxVal) * 100);
                                  const color = pct > 80 ? '#ff4d4f' : pct > 50 ? '#faad14' : '#52c41a';
                                  return (
                                    <Tooltip key={idx} title={`${m.value.toFixed(1)} @ ${new Date(m.timestamp * 1000).toLocaleTimeString()}`}>
                                      <div style={{ flex: 1, minWidth: 6, maxWidth: 24, background: color, borderRadius: 3, height: `${Math.max(4, pct * 0.4)}px`, transition: 'height 0.3s' }} />
                                    </Tooltip>
                                  );
                                })}
                              </div>
                              {/* Current values */}
                              <div style={{ marginTop: 8, display: 'flex', gap: 16 }}>
                                {pm.metrics.slice(-1).map((m, idx) => (
                                  <div key={idx}>
                                    <Text style={{ fontSize: 20, fontWeight: 600 }}>{m.value.toFixed(1)}</Text>
                                    <Text type="secondary" style={{ fontSize: 11, marginLeft: 4 }}>
                                      {selectedMetric === 'gpu_util' ? '%' : selectedMetric === 'fb_used' ? 'MiB' : selectedMetric === 'gpu_temp' ? '\u00b0C' : selectedMetric === 'power_usage' ? 'W' : '%'}
                                    </Text>
                                  </div>
                                ))}
                              </div>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <Alert type="info" message={metricsData.message || t('training.detail.metricsEmpty')} showIcon style={{ borderRadius: 8 }} />
                      )}
                    </div>
                  ) : (
                    <Alert type="warning" message={metricsData.message || t('training.detail.metricsUnavailable')} showIcon style={{ borderRadius: 8 }} />
                  )
                ) : (
                  <div style={{ textAlign: 'center', padding: 32 }}>
                    <Button onClick={() => { if (detailJob) fetchMetrics(detailJob, selectedMetric); }}>{t('training.detail.loadMetrics')}</Button>
                  </div>
                )}
              </div>
            ),
          },
          {
            key: 'yaml',
            label: 'YAML',
            children: (
              <pre style={{
                background: '#1e1e1e', color: '#d4d4d4', padding: 16, borderRadius: 8,
                fontSize: 11, fontFamily: 'SF Mono, Monaco, Menlo, monospace',
                lineHeight: 1.5, whiteSpace: 'pre-wrap', maxHeight: 500, overflow: 'auto', margin: '0 0 16px',
              }}>
                {detailYAML || 'Loading...'}
              </pre>
            ),
          },
          ...(detailJob?.kind === 'RayJob' ? [{
            key: 'dashboard',
            label: 'Ray Dashboard',
            children: (
              <div style={{ padding: '0 0 16px' }}>
                <div style={{ marginBottom: 12, padding: '12px 16px', background: '#f0f5ff', borderRadius: 8 }}>
                  <Text strong style={{ display: 'block', marginBottom: 4 }}>Ray Cluster Dashboard</Text>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    The Ray Dashboard provides real-time monitoring of tasks, actors, memory usage, and cluster resources.
                  </Text>
                </div>
                <iframe
                  src={`/ray/${detailJob.namespace}/${detailJob.name}/`}
                  style={{
                    width: '100%', height: 500, border: '1px solid #e8e8ed',
                    borderRadius: 8, background: '#fff',
                  }}
                  title="Ray Dashboard"
                />
                <div style={{ marginTop: 8 }}>
                  <Button type="link" href={`/ray/${detailJob.namespace}/${detailJob.name}/`} target="_blank">
                    Open in new tab
                  </Button>
                </div>
              </div>
            ),
          }] : []),
        ]} />
      </Drawer>

      {/* Create Training Job - Full Form */}
      <Modal
        title={<Space><ThunderboltOutlined />{t('training.create')}</Space>}
        open={createOpen}
        onOk={handleCreate}
        confirmLoading={submitting}
        onCancel={() => { setCreateOpen(false); form.resetFields(); }}
        okText={submitting ? 'Submitting...' : t('common.confirm')}
        cancelText={t('common.cancel')}
        width={780}
        styles={{ body: { maxHeight: '70vh', overflowY: 'auto' } }}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          {/* Basic Info */}
          <div style={{ background: '#f9f9fb', borderRadius: 12, padding: '16px 20px', marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 12 }}>{t('training.form.basicInfo')}</Text>
            <Row gutter={16}>
              <Col span={8}>
                <Form.Item name="name" label={t('training.name')} rules={[{ required: true, pattern: /^[a-z][a-z0-9-]*$/, message: 'lowercase, numbers, hyphens' }]}>
                  <Input placeholder="my-training-job" />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name="namespace" label="Namespace" initialValue="default-group">
                  <Select options={[
                    { label: 'default-group', value: 'default-group' },
                    { label: 'default', value: 'default' },
                  ]} />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name="kind" label={t('training.kind')} rules={[{ required: true }]} initialValue="PyTorchJob">
                  <Select onChange={(v) => setSelectedKind(v)} options={[
                    { label: 'PyTorchJob (DDP / DeepSpeed)', value: 'PyTorchJob' },
                    { label: 'TFJob (TensorFlow)', value: 'TFJob' },
                    { label: 'MPIJob (Horovod / DeepSpeed-MPI)', value: 'MPIJob' },
                    ...(raySupport ? [{ label: 'RayJob (KubeRay)', value: 'RayJob' }] : []),
                  ]} />
                </Form.Item>
              </Col>
            </Row>
            {experiments.length > 0 && (
              <Form.Item name="experiment" label={t('experiment.linkToTraining')} tooltip="Optionally link this training job to an experiment for tracking and comparison.">
                <Select allowClear placeholder={t('experiment.linkToTraining')} options={experiments.map((e) => ({ label: `${e.name} (${e.runCount} runs)`, value: e.name }))} />
              </Form.Item>
            )}
          </div>

          {/* Image & Command */}
          <div style={{ background: '#f9f9fb', borderRadius: 12, padding: '16px 20px', marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 12 }}>{t('training.form.imageCommand')}</Text>
            <Form.Item name="image" label={t('training.form.image')} rules={[{ required: true }]}>
              <Select
                showSearch
                allowClear
                placeholder={t('training.form.image.placeholder')}
                style={{ width: '100%' }}
                options={[
                  { label: 'PyTorch 2.3 + CUDA 12.1 (ACK)', value: 'registry.cn-hangzhou.aliyuncs.com/acs/pytorch:2.3-gpu-cuda12.1' },
                  { label: 'PyTorch 2.1 + CUDA 12.1 (ACK)', value: 'registry.cn-hangzhou.aliyuncs.com/acs/pytorch:2.1-gpu-cuda12.1' },
                  { label: 'PyTorch 2.1 + CUDA 11.8 (ACK)', value: 'registry.cn-hangzhou.aliyuncs.com/acs/pytorch:2.1-gpu-cuda11.8' },
                  { label: 'DeepSpeed + PyTorch 2.3', value: 'registry.cn-hangzhou.aliyuncs.com/acs/deepspeed:0.14-pytorch2.3-cuda12.1' },
                  { label: 'Megatron-LM (NVIDIA)', value: 'nvcr.io/nvidia/pytorch:24.01-py3' },
                  { label: 'Ray 2.10 + PyTorch (KubeRay)', value: 'rayproject/ray-ml:2.10.0-py310-gpu' },
                  { label: 'TensorFlow 2.15 GPU', value: 'tensorflow/tensorflow:2.15.0-gpu' },
                  { label: 'HuggingFace TRL (LLM fine-tuning)', value: 'huggingface/trl-latest-gpu:latest' },
                ]}
                dropdownRender={(menu) => (
                  <div>
                    {menu}
                    <Divider style={{ margin: '4px 0' }} />
                    <div style={{ padding: '4px 8px', fontSize: 11, color: '#999' }}>
                      Or type a custom image URL
                    </div>
                  </div>
                )}
                filterOption={(input, option) => (option?.label as string || '').toLowerCase().includes(input.toLowerCase()) || (option?.value as string || '').includes(input)}
              />
            </Form.Item>
            <Form.Item name="command" label={selectedKind === 'RayJob' ? t('training.form.entrypoint') : t('training.form.command')} rules={[{ required: true }]} tooltip={selectedKind === 'RayJob' ? 'Ray Job entrypoint script (e.g., python train.py --num-workers 4)' : 'Entry command. If you provide a script below, this will be overridden to run /scripts/train.sh'}>
              <Input.TextArea rows={3} placeholder={selectedKind === 'RayJob'
                ? 'python train.py --num-workers 4 --use-gpu'
                : 'torchrun --nproc_per_node=$GPU_PER_NODE --nnodes=$WORLD_SIZE train.py --lr 1e-4 --epochs 10'
              } style={{ fontFamily: 'monospace', fontSize: 12 }} />
            </Form.Item>
          </div>

          {/* Training Script (ConfigMap mount) */}
          <Collapse ghost style={{ marginBottom: 16 }} items={[{
            key: 'script',
            label: <Text strong style={{ fontSize: 13 }}>{t('training.form.script')}</Text>,
            children: (
              <div>
                <Form.Item name="script" tooltip="If provided, this script will be saved as a ConfigMap and mounted at /scripts/train.sh. The command will automatically run this script.">
                  <Input.TextArea
                    rows={10}
                    placeholder={`#!/bin/bash\nset -ex\n\n# Example: distributed training with torchrun\ntorchrun \\\\\n  --nproc_per_node=\\$GPU_PER_NODE \\\\\n  --nnodes=\\$WORLD_SIZE \\\\\n  --node_rank=\\$RANK \\\\\n  --master_addr=\\$MASTER_ADDR \\\\\n  --master_port=\\$MASTER_PORT \\\\\n  /workspace/train.py \\\\\n  --model_name_or_path /data/models/Qwen3-8B \\\\\n  --data_path /data/dataset/train.jsonl \\\\\n  --output_dir /data/output`}
                    style={{ fontFamily: 'SF Mono, Monaco, Menlo, monospace', fontSize: 12, lineHeight: 1.5 }}
                  />
                </Form.Item>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  {t('training.form.script.tip')}
                </Text>
              </div>
            ),
          }]} />

          {/* Distributed Config */}
          <div style={{ background: '#f9f9fb', borderRadius: 12, padding: '16px 20px', marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 12 }}>{t('training.form.resources')}</Text>
            {selectedKind !== 'RayJob' && (
              <Form.Item label={t('training.form.mode')} style={{ marginBottom: 12 }}>
                <Segmented
                  options={[
                    { label: t('training.form.mode.single'), value: 'single' },
                    { label: t('training.form.mode.distributed'), value: 'distributed' },
                  ]}
                  value={trainingMode}
                  onChange={(v) => setTrainingMode(v as string)}
                />
              </Form.Item>
            )}
            <Row gutter={16}>
              {(trainingMode === 'distributed' || selectedKind === 'RayJob') && (
                <Col span={6}>
                  <Form.Item name="workerCount" label={selectedKind === 'RayJob' ? t('training.form.rayWorkers') : t('training.form.workerCount')} initialValue={2}>
                    <InputNumber min={1} max={128} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
              )}
              <Col span={6}>
                <Form.Item name="workerCpu" label={t('training.form.cpuPerWorker')} initialValue="8">
                  <Input addonAfter="cores" />
                </Form.Item>
              </Col>
              <Col span={6}>
                <Form.Item name="workerMemory" label={t('training.form.memPerWorker')} initialValue="32Gi">
                  <Select options={[
                    { label: '16 GiB', value: '16Gi' },
                    { label: '32 GiB', value: '32Gi' },
                    { label: '64 GiB', value: '64Gi' },
                    { label: '128 GiB', value: '128Gi' },
                    { label: '256 GiB', value: '256Gi' },
                  ]} />
                </Form.Item>
              </Col>
              <Col span={6}>
                <Form.Item name="workerGpu" label={t('training.form.gpuPerWorker')} initialValue={1}>
                  <InputNumber min={0} max={8} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
            </Row>
            {selectedKind === 'RayJob' && (
              <div style={{ marginTop: 12, padding: '12px 16px', background: '#f0f5ff', borderRadius: 8 }}>
                <Text strong style={{ fontSize: 12, color: '#1677ff', display: 'block', marginBottom: 8 }}>{t('training.ray.config')}</Text>
                <Row gutter={16}>
                  <Col span={8}>
                    <Form.Item name="rayHeadCpu" label={t('training.ray.headCpu')} initialValue="2">
                      <Input addonAfter="cores" />
                    </Form.Item>
                  </Col>
                  <Col span={8}>
                    <Form.Item name="rayHeadMemory" label={t('training.ray.headMemory')} initialValue="8Gi">
                      <Select options={[
                        { label: '4 GiB', value: '4Gi' },
                        { label: '8 GiB', value: '8Gi' },
                        { label: '16 GiB', value: '16Gi' },
                      ]} />
                    </Form.Item>
                  </Col>
                  <Col span={8}>
                    <Form.Item name="shutdownAfterJob" label={t('training.ray.autoShutdown')} initialValue={true} valuePropName="checked">
                      <Switch />
                    </Form.Item>
                  </Col>
                </Row>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  {t('training.ray.dashboardHint')}
                </Text>
              </div>
            )}
          </div>

          {/* Data Mounts */}
          <Collapse ghost style={{ marginBottom: 16 }} items={[{
            key: 'data',
            label: <Text strong style={{ fontSize: 13 }}>{t('training.form.dataMounts')}</Text>,
            children: (
              <div>
                <Row gutter={16}>
                  <Col span={12}>
                    <Form.Item name="dataPvc" label="PVC Name">
                      <Input placeholder="my-dataset-pvc" />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item name="dataMountPath" label="Mount Path" initialValue="/data">
                      <Input placeholder="/data" />
                    </Form.Item>
                  </Col>
                </Row>
                <Form.Item name="shmSize" label="/dev/shm Size" initialValue="64Gi" tooltip="Shared memory for PyTorch DataLoader workers">
                  <Select options={[
                    { label: '16 GiB', value: '16Gi' },
                    { label: '32 GiB', value: '32Gi' },
                    { label: '64 GiB', value: '64Gi' },
                    { label: '128 GiB', value: '128Gi' },
                  ]} />
                </Form.Item>
              </div>
            ),
          }]} />

          {/* Environment Variables */}
          <Collapse ghost style={{ marginBottom: 16 }} items={[{
            key: 'env',
            label: <Text strong style={{ fontSize: 13 }}>{t('training.form.envVars')}</Text>,
            children: (
              <div>
                <Form.Item name="envVars" tooltip="One per line: KEY=VALUE">
                  <Input.TextArea rows={4} placeholder={'NCCL_DEBUG=WARN\nNCCL_IB_DISABLE=0\nNCCL_NET_GDR_LEVEL=2\nMASTER_PORT=29500'} style={{ fontFamily: 'monospace', fontSize: 12 }} />
                </Form.Item>
                <Space wrap>
                  <Button size="small" onClick={() => {
                    const current = form.getFieldValue('envVars') || '';
                    form.setFieldsValue({ envVars: current + '\nNCCL_DEBUG=INFO\nNCCL_IB_GID_INDEX=3\nNCCL_IB_SL=5\nNCCL_SOCKET_IFNAME=eth0\nNCCL_IB_TC=136\nNCCL_IB_HCA=mlx5\nNCCL_IB_QPS_PER_CONNECTION=8\nNCCL_NET_PLUGIN=none\nNCCL_IB_TIMEOUT=22' });
                  }}>+ RDMA/IB Full Preset</Button>
                  <Button size="small" onClick={() => {
                    const current = form.getFieldValue('envVars') || '';
                    form.setFieldsValue({ envVars: current + '\nNCCL_DEBUG=WARN\nNCCL_IB_DISABLE=0\nNCCL_NET_GDR_LEVEL=2' });
                  }}>+ NCCL Basic</Button>
                  <Button size="small" onClick={() => {
                    const current = form.getFieldValue('envVars') || '';
                    form.setFieldsValue({ envVars: current + '\nMASTER_PORT=29500\nTORCH_DISTRIBUTED_DEBUG=DETAIL\nTORCH_NCCL_ASYNC_ERROR_HANDLING=1' });
                  }}>+ PyTorch DDP</Button>
                  <Button size="small" onClick={() => {
                    const current = form.getFieldValue('envVars') || '';
                    form.setFieldsValue({ envVars: current + '\nDEEPSPEED_TIMEOUT=300\nNVIDIA_VISIBLE_DEVICES=all\nCUDA_DEVICE_MAX_CONNECTIONS=1' });
                  }}>+ DeepSpeed</Button>
                </Space>
              </div>
            ),
          }]} />

          {/* Advanced: RDMA, Scheduling */}
          <Collapse ghost items={[{
            key: 'advanced',
            label: <Text strong style={{ fontSize: 13 }}>{t('training.form.advanced')}</Text>,
            children: (
              <div>
                <Row gutter={16}>
                  <Col span={12}>
                    <Form.Item name="enableRdma" label={t('training.form.enableRdma')} valuePropName="checked">
                      <Switch />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item name="hostNetwork" label={t('training.form.hostNetwork')} valuePropName="checked">
                      <Switch />
                    </Form.Item>
                  </Col>
                </Row>
                <Divider style={{ margin: '8px 0' }} />
                <Row gutter={16}>
                  <Col span={12}>
                    <Form.Item name="queue" label={t('training.form.queue')}>
                      <Input placeholder="root.defaultQuotaGroup" />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item name="priority" label={t('training.form.priority')} initialValue="medium">
                      <Select options={[
                        { label: 'High', value: 'high' },
                        { label: 'Medium', value: 'medium' },
                        { label: 'Low', value: 'low' },
                      ]} />
                    </Form.Item>
                  </Col>
                </Row>
              </div>
            ),
          }]} />
        </Form>
      </Modal>
    </div>
  );
};

export default Training;
