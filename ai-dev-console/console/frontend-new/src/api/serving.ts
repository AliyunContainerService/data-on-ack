import { get, post, del } from './client';

export interface ServingInfo {
  name: string;
  namespace: string;
  status: string;
  replicas: string;
  endpoint: string;
  createTime: string;
  framework: string;
}

export interface ServingSpec {
  name: string;
  namespace: string;
  image: string;
  command?: string;
  replicas: number;
  cpu: string;
  memory: string;
  gpu: number;
  modelPath?: string;
  port: number;
  env?: Record<string, string>;
  framework?: string;
}

/** Inference engine options — LLM era */
export interface InferenceEngine {
  id: string;
  name: string;
  description: string;
  defaultImage: string;
  defaultPort: number;
  envHints: Record<string, string>;
  supportsModelPath: boolean;
}

export const INFERENCE_ENGINES: InferenceEngine[] = [
  {
    id: 'vllm',
    name: 'vLLM',
    description: 'High-throughput LLM serving with PagedAttention, continuous batching',
    defaultImage: 'vllm/vllm-openai:latest',
    defaultPort: 8000,
    envHints: { VLLM_MODEL: '/models', VLLM_TENSOR_PARALLEL_SIZE: '1' },
    supportsModelPath: true,
  },
  {
    id: 'sglang',
    name: 'SGLang',
    description: 'Fast serving with RadixAttention, structured generation, multi-modal support',
    defaultImage: 'lmsysorg/sglang:latest',
    defaultPort: 30000,
    envHints: { MODEL_PATH: '/models', TP_SIZE: '1' },
    supportsModelPath: true,
  },
  {
    id: 'tgi',
    name: 'TGI (Text Generation Inference)',
    description: 'HuggingFace optimized inference with quantization, flash attention',
    defaultImage: 'ghcr.io/huggingface/text-generation-inference:latest',
    defaultPort: 80,
    envHints: { MODEL_ID: '/models', QUANTIZE: 'awq' },
    supportsModelPath: true,
  },
  {
    id: 'triton',
    name: 'Triton Inference Server',
    description: 'NVIDIA multi-framework inference with dynamic batching, model ensemble',
    defaultImage: 'nvcr.io/nvidia/tritonserver:24.01-py3',
    defaultPort: 8000,
    envHints: {},
    supportsModelPath: true,
  },
  {
    id: 'torchserve',
    name: 'TorchServe',
    description: 'PyTorch native serving with model archiver, metrics, A/B testing',
    defaultImage: 'pytorch/torchserve:latest-gpu',
    defaultPort: 8080,
    envHints: {},
    supportsModelPath: true,
  },
  {
    id: 'tfserving',
    name: 'TensorFlow Serving',
    description: 'Production TensorFlow model serving with batching and versioning',
    defaultImage: 'tensorflow/serving:latest-gpu',
    defaultPort: 8501,
    envHints: {},
    supportsModelPath: true,
  },
  {
    id: 'custom',
    name: 'Custom Container',
    description: 'Bring your own inference container image',
    defaultImage: '',
    defaultPort: 8080,
    envHints: {},
    supportsModelPath: false,
  },
];

export function listServing(): Promise<ServingInfo[]> {
  return get<ServingInfo[]>('/serving');
}

export function getServing(namespace: string, name: string): Promise<ServingInfo> {
  return get<ServingInfo>(`/serving/${namespace}/${name}`);
}

export function createServing(spec: ServingSpec): Promise<unknown> {
  return post('/serving', spec);
}

export function deleteServing(namespace: string, name: string): Promise<unknown> {
  return del(`/serving/${namespace}/${name}`);
}

export interface ChatMessage {
  role: 'user' | 'assistant' | 'system';
  content: string;
}

export interface ChatRequest {
  model?: string;
  messages: ChatMessage[];
  max_tokens?: number;
  temperature?: number;
  stream?: boolean;
}

export function testServingEndpoint(namespace: string, name: string, req: ChatRequest): Promise<Record<string, unknown>> {
  return post<Record<string, unknown>>(`/serving/${namespace}/${name}/test`, req);
}

export interface PodInfo {
  name: string;
  namespace: string;
  status: string;
  node: string;
  ip: string;
  restarts: number;
  role: string;
  age: string;
}

export function listServingPods(namespace: string, name: string): Promise<PodInfo[]> {
  return get<PodInfo[]>(`/serving/${namespace}/${name}/pods`);
}

export function getServingLogs(namespace: string, name: string, pod: string, tail?: number): Promise<string> {
  const params: Record<string, string> = { pod };
  if (tail) params.tail = String(tail);
  return get<string>(`/serving/${namespace}/${name}/logs`, params);
}
