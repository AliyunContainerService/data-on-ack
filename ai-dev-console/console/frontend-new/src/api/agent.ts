import { get, post, del } from './client';

// --- Agent status ---

export interface AgentStatus {
  enabled: boolean;
  provider?: string;
  model?: string;
}

export function getAgentStatus(): Promise<AgentStatus> {
  return get<AgentStatus>('/agent/status');
}

// --- Sessions ---

export type AgentType = 'copilot' | 'diagnose';

export interface AgentSession {
  namespace: string;
  name: string;
  agentType: AgentType;
  title: string;
  createdAt: string;
}

export interface CreateSessionRequest {
  namespace?: string;
  name: string;
  agentType: AgentType;
  title: string;
}

export function listAgentSessions(): Promise<AgentSession[]> {
  return get<AgentSession[]>('/agent/sessions');
}

export function createAgentSession(req: CreateSessionRequest): Promise<{ namespace: string; name: string }> {
  return post<{ namespace: string; name: string }>('/agent/sessions', req);
}

export function deleteAgentSession(namespace: string, name: string): Promise<unknown> {
  return del(`/agent/sessions/${namespace}/${name}`);
}

// --- Messages / runs ---

export function sendSessionMessage(namespace: string, name: string, content: string): Promise<{ runID: string }> {
  return post<{ runID: string }>(`/agent/sessions/${namespace}/${name}/messages`, { content });
}

export function resolveConfirmation(runID: string, confirmationID: string, approved: boolean): Promise<unknown> {
  return post(`/agent/runs/${runID}/confirmations/${confirmationID}`, { approved });
}

export function stopRun(runID: string): Promise<unknown> {
  return post(`/agent/runs/${runID}/stop`);
}

// --- SSE event stream ---

export type AgentEventType =
  | 'run_started'
  | 'thinking'
  | 'text'
  | 'tool_call'
  | 'tool_result'
  | 'confirmation_request'
  | 'confirmation_resolved'
  | 'budget_exceeded'
  | 'error'
  | 'done';

export interface AgentEvent {
  seq: number;
  type: AgentEventType;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  data?: any;
}

export interface StreamRunEventsOptions {
  /** Called with the retry attempt (1-based) when reconnecting after a dropped stream. */
  onReconnecting?: (attempt: number) => void;
  /** Max reconnect attempts after an unexpected disconnect (default 3). */
  maxRetries?: number;
}

export interface StreamRunEventsHandle {
  abort: () => void;
  /** Resolves when the stream finished (terminal `done` event or abort); rejects on unrecoverable errors. */
  done: Promise<void>;
}

const MAX_RECONNECT_RETRIES = 3;

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Stream run events via fetch + ReadableStream (EventSource is not used: we
 * need cookie auth fine control and reconnect with a dynamic `after` cursor).
 * Frames look like `data: {"seq":1,"type":"text","data":{...}}\n\n`.
 * On an unexpected disconnect the stream is resumed from the last received
 * seq, up to `maxRetries` times.
 */
export function streamRunEvents(
  runID: string,
  after: number,
  onEvent: (ev: AgentEvent) => void,
  signal?: AbortSignal,
  opts?: StreamRunEventsOptions,
): StreamRunEventsHandle {
  const controller = new AbortController();
  const maxRetries = opts?.maxRetries ?? MAX_RECONNECT_RETRIES;
  let aborted = false;
  let terminated = false;
  let lastSeq = after;

  const onExternalAbort = () => {
    aborted = true;
    controller.abort();
  };
  if (signal) {
    if (signal.aborted) onExternalAbort();
    else signal.addEventListener('abort', onExternalAbort);
  }

  const handleFrame = (frame: string) => {
    for (const line of frame.split('\n')) {
      if (!line.startsWith('data:')) continue;
      const payload = line.slice(5).trim();
      if (!payload) continue;
      try {
        const ev = JSON.parse(payload) as AgentEvent;
        if (typeof ev.seq === 'number') lastSeq = Math.max(lastSeq, ev.seq);
        onEvent(ev);
        if (ev.type === 'done') terminated = true;
      } catch {
        // Ignore malformed frames; the server only emits valid JSON.
      }
    }
  };

  const run = async () => {
    let retries = 0;
    while (!aborted && !terminated) {
      let retryable = true;
      try {
        const resp = await fetch(`/api/v1/agent/runs/${encodeURIComponent(runID)}/events?after=${lastSeq}`, {
          method: 'GET',
          credentials: 'include',
          headers: { Accept: 'text/event-stream' },
          signal: controller.signal,
        });
        if (!resp.ok || !resp.body) {
          // 4xx (e.g. run not found / unauthorized) is not recoverable by retrying.
          retryable = resp.status >= 500;
          throw new Error(`events stream failed: HTTP ${resp.status}`);
        }
        const reader = resp.body.getReader();
        const decoder = new TextDecoder();
        let buf = '';
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;
          buf += decoder.decode(value, { stream: true });
          let sep: number;
          while ((sep = buf.indexOf('\n\n')) >= 0) {
            const frame = buf.slice(0, sep);
            buf = buf.slice(sep + 2);
            handleFrame(frame);
            if (terminated) break;
          }
          if (terminated) break;
        }
        if (terminated || aborted) return;
        // The server closed the stream without a terminal event: reconnect.
        retryable = true;
        throw new Error('events stream ended unexpectedly');
      } catch (err) {
        if (aborted || terminated) return;
        if (!retryable || retries >= maxRetries) throw err;
        retries += 1;
        opts?.onReconnecting?.(retries);
        await sleep(500 * retries);
      }
    }
  };

  const done = run().finally(() => {
    if (signal) signal.removeEventListener('abort', onExternalAbort);
  });

  return {
    abort: () => {
      aborted = true;
      controller.abort();
    },
    done,
  };
}
