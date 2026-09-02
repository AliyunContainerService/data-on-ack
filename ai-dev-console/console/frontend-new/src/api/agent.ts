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
  /**
   * Max reconnect attempts after an unexpected disconnect (default 3).
   * The budget is replenished: every successfully received event resets the
   * used attempts back to zero, so long sessions (waiting for confirmations,
   * long tool executions) survive repeated network blips.
   */
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
 * On an unexpected disconnect — or when a seq gap is detected in the received
 * frames — the stream is resumed from the last received seq (`?after=<lastSeq>`),
 * up to `maxRetries` times. The retry budget resets to zero after every
 * successfully received event.
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
  // Reconnect budget: incremented on each retry, reset to zero whenever an
  // event is successfully consumed (see handleFrame).
  let retries = 0;
  // Set when a seq gap is detected: the current stream stops being consumed
  // and is resumed from `lastSeq` via the regular reconnect path.
  let gapDetected = false;

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
        if (typeof ev.seq === 'number') {
          if (ev.seq > lastSeq + 1) {
            // Missing frame(s): stop consuming this stream; the reconnect
            // below resumes from the last contiguous seq (counts as one retry).
            gapDetected = true;
            return;
          }
          lastSeq = Math.max(lastSeq, ev.seq);
        }
        onEvent(ev);
        // Event consumed successfully: replenish the reconnect budget so long
        // sessions are not permanently cut off by later network blips.
        retries = 0;
        if (ev.type === 'done') terminated = true;
      } catch {
        // Ignore malformed frames; the server only emits valid JSON.
      }
    }
  };

  const run = async () => {
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
            if (terminated || gapDetected) break;
          }
          if (terminated || gapDetected) break;
        }
        if (terminated || aborted) return;
        if (gapDetected) {
          // Seq gap in the current stream: drop it and resume from the last
          // contiguous seq via the reconnect path (counts as one retry).
          gapDetected = false;
          reader.cancel().catch(() => {});
          throw new Error(`events stream seq gap detected after ${lastSeq}`);
        }
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
