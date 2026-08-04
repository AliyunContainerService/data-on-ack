import { get, post, del } from './client';

export interface ClusterCapabilities {
  raySupport: boolean;
}

export function getCapabilities(): Promise<ClusterCapabilities> {
  return get<ClusterCapabilities>('/capabilities');
}

export interface TrainingJobInfo {
  name: string;
  namespace: string;
  kind: string;
  status: string;
  reason?: string;
  message?: string;
  duration: string;
  createTime: string;
  gpu: number;
}

export interface TrainingJobSpec {
  name: string;
  namespace: string;
  kind: string;
  image: string;
  command: string;
  script?: string;
  workerCount: number;
  workerCpu: string;
  workerMemory: string;
  workerGpu: number;
  psCount?: number;
  psCpu?: string;
  psMemory?: string;
  env?: Record<string, string>;
  dataSources?: { name: string; mountPath: string }[];
  gpuType?: string;
  shmSize?: string;
  enableRdma?: boolean;
  hostNetwork?: boolean;
  queue?: string;
  priority?: string;
}

export function listTrainingJobs(kind?: string): Promise<TrainingJobInfo[]> {
  const params = kind ? { kind } : undefined;
  return get<TrainingJobInfo[]>('/training-jobs', params);
}

export function getTrainingJob(namespace: string, name: string, kind?: string): Promise<TrainingJobInfo> {
  const params = kind ? { kind } : undefined;
  return get<TrainingJobInfo>(`/training-jobs/${namespace}/${name}`, params);
}

export function createTrainingJob(spec: TrainingJobSpec): Promise<unknown> {
  return post('/training-jobs', spec);
}

export function deleteTrainingJob(namespace: string, name: string, kind?: string): Promise<unknown> {
  return del(`/training-jobs/${namespace}/${name}${kind ? '?kind=' + kind : ''}`);
}

export interface PodInfo {
  name: string;
  namespace: string;
  status: string;
  reason?: string;
  message?: string;
  node: string;
  ip: string;
  restarts: number;
  role: string;
  age: string;
}

export function listTrainingJobPods(namespace: string, name: string, kind?: string): Promise<PodInfo[]> {
  const params: Record<string, string> = {};
  if (kind) params.kind = kind;
  return get<PodInfo[]>(`/training-jobs/${namespace}/${name}/pods`, params);
}

export function getTrainingJobLogs(namespace: string, name: string, pod: string, tail?: number, container?: string): Promise<string> {
  const params: Record<string, string> = { pod };
  if (tail) params.tail = String(tail);
  if (container) params.container = container;
  return get<string>(`/training-jobs/${namespace}/${name}/logs`, params);
}

export interface EventInfo {
  type: string;
  reason: string;
  message: string;
  object: string;
  time: string;
  count: number;
}

export function getTrainingJobEvents(namespace: string, name: string): Promise<EventInfo[]> {
  return get<EventInfo[]>(`/training-jobs/${namespace}/${name}/events`);
}

export function getTrainingJobYAML(namespace: string, name: string, kind?: string): Promise<Record<string, unknown>> {
  const params: Record<string, string> = {};
  if (kind) params.kind = kind;
  return get<Record<string, unknown>>(`/training-jobs/${namespace}/${name}/yaml`, params);
}

// --- GPU Metrics ---

export interface MetricPoint {
  timestamp: number;
  value: number;
}

export interface PodMetrics {
  pod: string;
  node: string;
  gpu: number;
  metrics: MetricPoint[];
}

export interface GPUMetricsResponse {
  available: boolean;
  message?: string;
  pods?: PodMetrics[];
}

export function getTrainingJobMetrics(namespace: string, name: string, metric?: string, kind?: string): Promise<GPUMetricsResponse> {
  const params: Record<string, string> = {};
  if (metric) params.metric = metric;
  if (kind) params.kind = kind;
  return get<GPUMetricsResponse>(`/training-jobs/${namespace}/${name}/metrics`, params);
}

export function getAvailableMetrics(): Promise<string[]> {
  return get<string[]>('/metrics/available');
}

// --- Experiments ---

export interface ExperimentInfo {
  name: string;
  namespace: string;
  description: string;
  createdBy: string;
  createTime: string;
  runCount: number;
  bestLoss?: number;
  runs?: ExperimentRun[];
}

export interface ExperimentRun {
  jobName: string;
  jobKind: string;
  params?: Record<string, string>;
  metrics?: Record<string, number>;
  status: string;
  addedAt: string;
}

export interface ExperimentCreateSpec {
  name: string;
  namespace: string;
  description: string;
}

export interface ExperimentRunSpec {
  jobName: string;
  jobKind: string;
  params?: Record<string, string>;
  metrics?: Record<string, number>;
  status: string;
}

export function listExperiments(): Promise<ExperimentInfo[]> {
  return get<ExperimentInfo[]>('/experiments');
}

export function getExperiment(name: string, namespace?: string): Promise<ExperimentInfo> {
  const params: Record<string, string> = {};
  if (namespace) params.namespace = namespace;
  return get<ExperimentInfo>(`/experiments/${name}`, params);
}

export function createExperiment(spec: ExperimentCreateSpec): Promise<unknown> {
  return post('/experiments', spec);
}

export function addExperimentRun(experimentName: string, run: ExperimentRunSpec, namespace?: string): Promise<unknown> {
  const params = namespace ? `?namespace=${namespace}` : '';
  return post(`/experiments/${experimentName}/runs${params}`, run);
}

export function deleteExperiment(name: string, namespace?: string): Promise<unknown> {
  const params = namespace ? `?namespace=${namespace}` : '';
  return del(`/experiments/${name}${params}`);
}
