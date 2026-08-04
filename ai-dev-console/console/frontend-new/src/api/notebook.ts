import { get, post, put, del } from './client';

export interface NotebookInfo {
  name: string;
  namespace: string;
  image: string;
  status: string;
  cpu: string;
  memory: string;
  gpu: string;
  age: string;
  url?: string;
}

export interface NotebookSpec {
  name: string;
  namespace: string;
  image: string;
  cpu: string;
  memory: string;
  gpu: number;
  gpuType?: string;
  storage?: string;
  env?: Record<string, string>;
}

/** Pre-defined notebook templates for one-click launch */
export interface NotebookTemplate {
  id: string;
  name: string;
  description: string;
  icon: string;
  image: string;
  cpu: string;
  memory: string;
  gpu: number;
  gpuType?: string;
  storage: string;
  tags: string[];
  env?: Record<string, string>;
}

export const NOTEBOOK_TEMPLATES: NotebookTemplate[] = [
  {
    id: 'pytorch-gpu',
    name: 'PyTorch 2.x GPU',
    description: 'PyTorch 2.x with CUDA 12.1, Jupyter, common ML libraries pre-installed',
    icon: 'fire',
    image: 'registry.cn-hangzhou.aliyuncs.com/acs/jupyter-pytorch:2.1-gpu-cuda12.1',
    cpu: '4',
    memory: '16Gi',
    gpu: 1,
    storage: '50Gi',
    tags: ['GPU', 'PyTorch', 'CUDA 12.1'],
    env: { JUPYTER_ENABLE_LAB: 'yes' },
  },
  {
    id: 'tensorflow-gpu',
    name: 'TensorFlow 2.x GPU',
    description: 'TensorFlow 2.x with CUDA, Keras, TensorBoard pre-installed',
    icon: 'experiment',
    image: 'registry.cn-hangzhou.aliyuncs.com/acs/jupyter-tensorflow:2.14-gpu-cuda12.1',
    cpu: '4',
    memory: '16Gi',
    gpu: 1,
    storage: '50Gi',
    tags: ['GPU', 'TensorFlow', 'CUDA 12.1'],
    env: { JUPYTER_ENABLE_LAB: 'yes' },
  },
  {
    id: 'llm-dev',
    name: 'LLM Development',
    description: 'Transformers, PEFT, DeepSpeed, vLLM, bitsandbytes for LLM development',
    icon: 'robot',
    image: 'registry.cn-hangzhou.aliyuncs.com/acs/jupyter-llm:latest',
    cpu: '8',
    memory: '32Gi',
    gpu: 1,
    gpuType: 'nvidia.com/gpu',
    storage: '100Gi',
    tags: ['GPU', 'LLM', 'Transformers', 'PEFT'],
    env: { JUPYTER_ENABLE_LAB: 'yes', HF_HOME: '/home/jovyan/huggingface' },
  },
  {
    id: 'scipy-cpu',
    name: 'Data Science (CPU)',
    description: 'SciPy, Pandas, Scikit-learn, Matplotlib for data analysis',
    icon: 'barChart',
    image: 'registry.cn-hangzhou.aliyuncs.com/acs/jupyter-scipy:latest',
    cpu: '2',
    memory: '8Gi',
    gpu: 0,
    storage: '20Gi',
    tags: ['CPU', 'SciPy', 'Pandas'],
    env: { JUPYTER_ENABLE_LAB: 'yes' },
  },
  {
    id: 'multimodal',
    name: 'Multimodal AI',
    description: 'Diffusers, Qwen-VL, CLIP, Stable Diffusion for vision-language tasks',
    icon: 'picture',
    image: 'registry.cn-hangzhou.aliyuncs.com/acs/jupyter-multimodal:latest',
    cpu: '8',
    memory: '32Gi',
    gpu: 1,
    storage: '100Gi',
    tags: ['GPU', 'Multimodal', 'Diffusers'],
    env: { JUPYTER_ENABLE_LAB: 'yes' },
  },
  {
    id: 'rag-dev',
    name: 'RAG Development',
    description: 'LangChain, LlamaIndex, vector databases, embedding models for RAG pipelines',
    icon: 'database',
    image: 'registry.cn-hangzhou.aliyuncs.com/acs/jupyter-rag:latest',
    cpu: '4',
    memory: '16Gi',
    gpu: 0,
    storage: '50Gi',
    tags: ['CPU', 'RAG', 'LangChain', 'LlamaIndex'],
    env: { JUPYTER_ENABLE_LAB: 'yes' },
  },
];

/** Resource presets for quick selection */
export interface ResourcePreset {
  id: string;
  label: string;
  cpu: string;
  memory: string;
  gpu: number;
  description: string;
}

export const RESOURCE_PRESETS: ResourcePreset[] = [
  { id: 'small-cpu', label: '2C/4G', cpu: '2', memory: '4Gi', gpu: 0, description: 'Light exploration' },
  { id: 'medium-cpu', label: '4C/8G', cpu: '4', memory: '8Gi', gpu: 0, description: 'Data processing' },
  { id: 'large-cpu', label: '8C/32G', cpu: '8', memory: '32Gi', gpu: 0, description: 'CPU-intensive' },
  { id: 'gpu-1', label: '4C/16G/1GPU', cpu: '4', memory: '16Gi', gpu: 1, description: 'Single GPU training' },
  { id: 'gpu-2', label: '8C/32G/2GPU', cpu: '8', memory: '32Gi', gpu: 2, description: 'Multi-GPU dev' },
  { id: 'gpu-4', label: '16C/64G/4GPU', cpu: '16', memory: '64Gi', gpu: 4, description: 'Large model dev' },
  { id: 'gpu-8', label: '32C/128G/8GPU', cpu: '32', memory: '128Gi', gpu: 8, description: 'Full node' },
];

export function listNotebooks(): Promise<NotebookInfo[]> {
  return get<NotebookInfo[]>('/notebooks');
}

export function getNotebook(namespace: string, name: string): Promise<NotebookInfo> {
  return get<NotebookInfo>(`/notebooks/${namespace}/${name}`);
}

export function createNotebook(spec: NotebookSpec): Promise<unknown> {
  return post('/notebooks', spec);
}

export function deleteNotebook(namespace: string, name: string): Promise<unknown> {
  return del(`/notebooks/${namespace}/${name}`);
}

export function stopNotebook(namespace: string, name: string): Promise<unknown> {
  return put(`/notebooks/${namespace}/${name}/stop`);
}

export function startNotebook(namespace: string, name: string): Promise<unknown> {
  return put(`/notebooks/${namespace}/${name}/start`);
}

export interface NotebookSSHInfo {
  podName: string;
  namespace: string;
  node: string;
  ip: string;
  portForward: string;
  execCommand: string;
}

export function getNotebookSSHInfo(namespace: string, name: string): Promise<NotebookSSHInfo> {
  return get<NotebookSSHInfo>(`/notebooks/${namespace}/${name}/ssh`);
}

export interface ResizeSpec {
  cpu: string;
  memory: string;
  gpu: number;
}

export function resizeNotebook(namespace: string, name: string, spec: ResizeSpec): Promise<unknown> {
  return put(`/notebooks/${namespace}/${name}/resize`, spec);
}
