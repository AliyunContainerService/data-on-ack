import type { AgentEvent } from '../../api/agent';

export interface ConfirmationInfo {
  id: string;
  resolved: boolean;
  approved?: boolean;
  submitting?: boolean;
}

export interface ToolCallInfo {
  /** toolCallID (falls back to a synthetic key when absent). */
  key: string;
  name: string;
  input?: unknown;
  status: 'running' | 'awaiting_confirmation' | 'succeeded' | 'failed';
  resultState?: string;
  resultText?: string;
  confirmation?: ConfirmationInfo;
}

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  /** Run id this assistant message belongs to (temp id until the POST returns). */
  runID?: string;
  thinking?: string;
  toolCalls?: ToolCallInfo[];
  streaming?: boolean;
  error?: string;
  budgetExceeded?: string;
  doneStatus?: string;
}

export function uid(): string {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

function upsertToolCall(list: ToolCallInfo[], patch: Partial<ToolCallInfo> & { key: string; name: string }): ToolCallInfo[] {
  const idx = list.findIndex((tc) => tc.key === patch.key);
  if (idx < 0) return [...list, { status: 'running', ...patch }];
  const next = [...list];
  next[idx] = { ...next[idx], ...patch };
  return next;
}

/**
 * Pure reducer folding one SSE event into an assistant message.
 * Only plain-text / structured data is stored; rendering never uses HTML.
 */
export function reduceMessage(msg: ChatMessage, ev: AgentEvent): ChatMessage {
  const data = ev.data ?? {};
  switch (ev.type) {
    case 'thinking':
      return { ...msg, thinking: (msg.thinking ?? '') + (typeof data.text === 'string' ? data.text : '') };
    case 'text':
      return { ...msg, content: msg.content + (typeof data.text === 'string' ? data.text : '') };
    case 'tool_call': {
      const key = typeof data.toolCallID === 'string' ? data.toolCallID : `call-${ev.seq}`;
      const name = typeof data.tool === 'string' ? data.tool : typeof data.name === 'string' ? data.name : 'tool';
      const toolCalls = upsertToolCall(msg.toolCalls ?? [], {
        key,
        name,
        input: data.input ?? data.args,
        status: 'running',
      });
      return { ...msg, toolCalls };
    }
    case 'tool_result': {
      const key = typeof data.toolCallID === 'string' ? data.toolCallID : '';
      const list = msg.toolCalls ?? [];
      const idx = list.findIndex((tc) => tc.key === key);
      const state = typeof data.state === 'string' ? data.state : '';
      const failed = state.toLowerCase().includes('error') || state.toLowerCase() === 'failed';
      const text = typeof data.text === 'string' ? data.text : '';
      if (idx < 0) {
        return {
          ...msg,
          toolCalls: [
            ...list,
            {
              key: key || `result-${ev.seq}`,
              name: typeof data.name === 'string' ? data.name : 'tool',
              status: failed ? 'failed' : 'succeeded',
              resultState: state,
              resultText: text,
            },
          ],
        };
      }
      const next = [...list];
      next[idx] = { ...next[idx], status: failed ? 'failed' : 'succeeded', resultState: state, resultText: text };
      return { ...msg, toolCalls: next };
    }
    case 'confirmation_request': {
      const items: unknown[] = Array.isArray(data.confirmations) ? data.confirmations : [];
      let toolCalls = msg.toolCalls ?? [];
      for (const raw of items) {
        if (!raw || typeof raw !== 'object') continue;
        const item = raw as Record<string, unknown>;
        const cfmID = typeof item.confirmationID === 'string' ? item.confirmationID : '';
        if (!cfmID) continue;
        const toolCallID = typeof item.toolCallID === 'string' ? item.toolCallID : `cfm-${cfmID}`;
        const toolName = typeof item.tool === 'string' ? item.tool : 'tool';
        toolCalls = upsertToolCall(toolCalls, {
          key: toolCallID,
          name: toolName,
          input: item.input,
          status: 'awaiting_confirmation',
          confirmation: { id: cfmID, resolved: false },
        });
      }
      return { ...msg, toolCalls };
    }
    case 'confirmation_resolved': {
      const cfmID = typeof data.confirmationID === 'string' ? data.confirmationID : '';
      const approved = data.approved === true;
      const toolCalls = (msg.toolCalls ?? []).map((tc) =>
        tc.confirmation && tc.confirmation.id === cfmID
          ? { ...tc, confirmation: { ...tc.confirmation, resolved: true, approved, submitting: false } }
          : tc,
      );
      return { ...msg, toolCalls };
    }
    case 'budget_exceeded':
      return { ...msg, budgetExceeded: typeof data.message === 'string' ? data.message : '' };
    case 'error':
      return { ...msg, error: typeof data.message === 'string' ? data.message : 'unknown error' };
    case 'done':
      return { ...msg, streaming: false, doneStatus: typeof data.status === 'string' ? data.status : '' };
    default:
      return msg;
  }
}

export function formatJSON(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}
