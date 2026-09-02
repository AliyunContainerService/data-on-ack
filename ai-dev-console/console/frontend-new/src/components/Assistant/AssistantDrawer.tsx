import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Alert, Button, Collapse, Drawer, Empty, Input, Popconfirm, Select, Space, Tag, Tooltip, Typography, message,
} from 'antd';
import {
  DeleteOutlined, LoadingOutlined, PlusOutlined, ReloadOutlined, RobotOutlined, SendOutlined, StopOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  AgentEvent, AgentSession, AgentType, StreamRunEventsHandle,
  createAgentSession, deleteAgentSession, listAgentSessions,
  resolveConfirmation, sendSessionMessage, stopRun, streamRunEvents,
} from '../../api/agent';
import { useAssistantStore } from '../../store/assistant';
import type { ChatMessage, ToolCallInfo } from './types';
import { reduceMessage, uid } from './types';
import ToolCallCard from './ToolCallCard';

const { Text } = Typography;

function sessionKey(s: { namespace: string; name: string }): string {
  return `${s.namespace}/${s.name}`;
}

function genSessionName(prefix: string): string {
  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`;
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

const AssistantDrawer: React.FC = () => {
  const { t } = useTranslation();
  const { open, setOpen, diagnoseRequest, clearDiagnoseRequest } = useAssistantStore();

  const [sessions, setSessions] = useState<AgentSession[]>([]);
  const [sessionsLoaded, setSessionsLoaded] = useState(false);
  const [loadingSessions, setLoadingSessions] = useState(false);
  const [activeKey, setActiveKey] = useState<string>('');
  const [messagesMap, setMessagesMap] = useState<Record<string, ChatMessage[]>>({});
  const [runningMap, setRunningMap] = useState<Record<string, string>>({});
  const [reconnecting, setReconnecting] = useState(false);
  const [input, setInput] = useState('');
  const [creating, setCreating] = useState(false);

  const streamHandlesRef = useRef<Map<string, StreamRunEventsHandle>>(new Map());
  const consumedDiagnoseSeqRef = useRef(0);
  const bottomRef = useRef<HTMLDivElement>(null);

  const activeSession = sessions.find((s) => sessionKey(s) === activeKey) ?? null;
  const messages = messagesMap[activeKey] ?? [];
  const currentRunID = runningMap[activeKey];
  const isRunning = Boolean(currentRunID);

  // --- message helpers -------------------------------------------------

  const patchMessage = useCallback((key: string, pred: (m: ChatMessage) => boolean, fn: (m: ChatMessage) => ChatMessage) => {
    setMessagesMap((prev) => {
      const list = prev[key];
      if (!list) return prev;
      const idx = list.findIndex(pred);
      if (idx < 0) return prev;
      const next = [...list];
      next[idx] = fn(next[idx]);
      return { ...prev, [key]: next };
    });
  }, []);

  const applyEvent = useCallback((key: string, runID: string, ev: AgentEvent) => {
    patchMessage(key, (m) => m.role === 'assistant' && m.runID === runID, (m) => reduceMessage(m, ev));
  }, [patchMessage]);

  // --- run lifecycle ----------------------------------------------------

  const startRun = useCallback(async (session: AgentSession, content: string) => {
    const key = sessionKey(session);
    const tempRunID = `pending-${uid()}`;
    const userMsg: ChatMessage = { id: uid(), role: 'user', content };
    const assistantMsg: ChatMessage = { id: uid(), role: 'assistant', content: '', runID: tempRunID, streaming: true };
    setMessagesMap((prev) => ({ ...prev, [key]: [...(prev[key] ?? []), userMsg, assistantMsg] }));

    let runID: string;
    try {
      const resp = await sendSessionMessage(session.namespace, session.name, content);
      runID = resp.runID;
    } catch (err) {
      patchMessage(key, (m) => m.runID === tempRunID, (m) => ({ ...m, streaming: false, error: errorMessage(err) }));
      return;
    }

    patchMessage(key, (m) => m.runID === tempRunID, (m) => ({ ...m, runID }));
    setRunningMap((prev) => ({ ...prev, [key]: runID }));
    setActiveKey(key);

    const handle = streamRunEvents(runID, 0, (ev) => applyEvent(key, runID, ev), undefined, {
      onReconnecting: () => setReconnecting(true),
    });
    streamHandlesRef.current.set(key, handle);

    const finish = () => {
      setReconnecting(false);
      setRunningMap((prev) => {
        if (prev[key] !== runID) return prev;
        const next = { ...prev };
        delete next[key];
        return next;
      });
      // Only remove our own handle, never a newer run's: the diagnose
      // in-flight guard (store) and the isRunning check in handleSend prevent
      // concurrent runs on the same session, but keep the delete guarded so
      // finish() can't drop a second request's handle even if one appears.
      if (streamHandlesRef.current.get(key) === handle) {
        streamHandlesRef.current.delete(key);
      }
    };

    handle.done.then(finish).catch((err) => {
      finish();
      patchMessage(key, (m) => m.runID === runID, (m) => ({
        ...m,
        streaming: false,
        error: m.error ?? errorMessage(err),
      }));
    });
  }, [applyEvent, patchMessage]);

  // --- session management -----------------------------------------------

  const doCreateSession = useCallback(async (agentType: AgentType, title: string): Promise<AgentSession> => {
    const name = genSessionName(agentType === 'diagnose' ? 'diag' : 'chat');
    const created = await createAgentSession({ name, agentType, title });
    const session: AgentSession = {
      namespace: created.namespace,
      name: created.name,
      agentType,
      title,
      createdAt: new Date().toISOString(),
    };
    setSessions((prev) => [session, ...prev]);
    setActiveKey(sessionKey(session));
    return session;
  }, []);

  const loadSessions = useCallback(async () => {
    setLoadingSessions(true);
    try {
      const list = await listAgentSessions();
      const sorted = [...list].sort((a, b) => (b.createdAt || '').localeCompare(a.createdAt || ''));
      setSessions(sorted);
      setActiveKey((prev) => prev || (sorted.length > 0 ? sessionKey(sorted[0]) : ''));
      return sorted;
    } catch (err) {
      message.error(errorMessage(err));
      return [];
    } finally {
      setLoadingSessions(false);
    }
  }, []);

  // Load sessions on first open; auto-create a default copilot session when none exist.
  useEffect(() => {
    if (!open || sessionsLoaded) return;
    let cancelled = false;
    (async () => {
      const list = await loadSessions();
      if (cancelled) return;
      setSessionsLoaded(true);
      if (list.length === 0) {
        setCreating(true);
        try {
          await doCreateSession('copilot', t('assistant.session.defaultTitle'));
        } catch (err) {
          message.error(errorMessage(err));
        } finally {
          if (!cancelled) setCreating(false);
        }
      }
    })();
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, sessionsLoaded]);

  const handleNewSession = async () => {
    if (creating) return;
    setCreating(true);
    try {
      await doCreateSession('copilot', t('assistant.session.defaultTitle'));
    } catch (err) {
      message.error(errorMessage(err));
    } finally {
      setCreating(false);
    }
  };

  const handleDeleteSession = async (target: AgentSession) => {
    const key = sessionKey(target);
    streamHandlesRef.current.get(key)?.abort();
    streamHandlesRef.current.delete(key);
    try {
      await deleteAgentSession(target.namespace, target.name);
      const remaining = sessions.filter((s) => sessionKey(s) !== key);
      setSessions(remaining);
      if (activeKey === key) {
        setActiveKey(remaining.length > 0 ? sessionKey(remaining[0]) : '');
      }
      setMessagesMap((prev) => {
        const next = { ...prev };
        delete next[key];
        return next;
      });
      setRunningMap((prev) => {
        const next = { ...prev };
        delete next[key];
        return next;
      });
    } catch (err) {
      message.error(errorMessage(err));
    }
  };

  // --- diagnose entry point (from Training pages) ------------------------

  useEffect(() => {
    if (!open || !diagnoseRequest || !sessionsLoaded) return;
    if (consumedDiagnoseSeqRef.current === diagnoseRequest.seq) return;
    consumedDiagnoseSeqRef.current = diagnoseRequest.seq;
    const target = diagnoseRequest;
    (async () => {
      try {
        let session = sessions.find((s) => s.agentType === 'diagnose');
        if (!session) {
          session = await doCreateSession('diagnose', t('assistant.diagnose.sessionTitle', { name: target.name }));
        } else {
          setActiveKey(sessionKey(session));
        }
        const prefill = t('assistant.diagnose.prefill', {
          namespace: target.namespace,
          name: target.name,
          kind: target.kind,
        });
        await startRun(session, prefill);
      } catch (err) {
        message.error(errorMessage(err));
      } finally {
        clearDiagnoseRequest();
      }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, diagnoseRequest, sessionsLoaded]);

  // --- user actions -------------------------------------------------------

  const handleSend = () => {
    const text = input.trim();
    if (!text || !activeSession || isRunning) return;
    setInput('');
    startRun(activeSession, text);
  };

  const handleStop = async () => {
    if (!currentRunID) return;
    try {
      await stopRun(currentRunID);
    } catch (err) {
      message.error(errorMessage(err));
    }
  };

  const handleAnswerConfirmation = async (msg: ChatMessage, tool: ToolCallInfo, approved: boolean) => {
    if (!msg.runID || !tool.confirmation) return;
    const key = activeKey;
    const cfmID = tool.confirmation.id;
    patchMessage(key, (m) => m.id === msg.id, (m) => ({
      ...m,
      toolCalls: (m.toolCalls ?? []).map((tc) =>
        tc.key === tool.key && tc.confirmation ? { ...tc, confirmation: { ...tc.confirmation, submitting: true } } : tc),
    }));
    try {
      await resolveConfirmation(msg.runID, cfmID, approved);
      // The server also emits confirmation_resolved; apply optimistically.
      patchMessage(key, (m) => m.id === msg.id, (m) => ({
        ...m,
        toolCalls: (m.toolCalls ?? []).map((tc) =>
          tc.key === tool.key && tc.confirmation
            ? { ...tc, confirmation: { ...tc.confirmation, resolved: true, approved, submitting: false } }
            : tc),
      }));
    } catch (err) {
      message.error(errorMessage(err));
      patchMessage(key, (m) => m.id === msg.id, (m) => ({
        ...m,
        toolCalls: (m.toolCalls ?? []).map((tc) =>
          tc.key === tool.key && tc.confirmation ? { ...tc, confirmation: { ...tc.confirmation, submitting: false } } : tc),
      }));
    }
  };

  // Abort client-side streams when the drawer unmounts (runs continue server-side).
  useEffect(() => () => {
    streamHandlesRef.current.forEach((h) => h.abort());
    streamHandlesRef.current.clear();
  }, []);

  // Auto-scroll to the newest content.
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' });
  }, [messagesMap, activeKey]);

  // --- rendering -----------------------------------------------------------

  const renderMessage = (msg: ChatMessage) => {
    if (msg.role === 'user') {
      return (
        <div key={msg.id} style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
          <div style={{
            maxWidth: '85%', background: '#0071e3', color: '#fff', borderRadius: 12,
            padding: '8px 12px', fontSize: 13, whiteSpace: 'pre-wrap', wordBreak: 'break-word',
          }}>
            {msg.content}
          </div>
        </div>
      );
    }

    const showTyping = msg.streaming && !msg.content && !msg.thinking && !(msg.toolCalls ?? []).length && !msg.error && !msg.budgetExceeded;

    return (
      <div key={msg.id} style={{ display: 'flex', justifyContent: 'flex-start', marginBottom: 12 }}>
        <div style={{
          maxWidth: '92%', background: '#ffffff', border: '1px solid #f0f0f3', borderRadius: 12,
          padding: '10px 12px', fontSize: 13, flex: 1,
        }}>
          {msg.thinking && (
            <Collapse
              ghost
              size="small"
              style={{ marginBottom: 4, background: '#fafafa', borderRadius: 8 }}
              items={[{
                key: 'thinking',
                label: <Text type="secondary" style={{ fontSize: 12 }}>{t('assistant.thinking')}</Text>,
                children: (
                  <div style={{ fontSize: 12, color: '#86868b', whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                    {msg.thinking}
                  </div>
                ),
              }]}
            />
          )}

          {showTyping && (
            <Space size={6}>
              <LoadingOutlined spin style={{ color: '#86868b' }} />
              <Text type="secondary" style={{ fontSize: 12 }}>{t('assistant.thinking')}</Text>
            </Space>
          )}

          {msg.content && (
            <div style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', lineHeight: 1.7 }}>
              {msg.content}
            </div>
          )}

          {(msg.toolCalls ?? []).map((tool) => (
            <ToolCallCard
              key={tool.key}
              tool={tool}
              onAnswerConfirmation={(tc, approved) => handleAnswerConfirmation(msg, tc, approved)}
            />
          ))}

          {msg.budgetExceeded && (
            <Alert
              type="warning"
              showIcon
              style={{ marginTop: 8, borderRadius: 8 }}
              message={t('assistant.budgetExceeded')}
              description={msg.budgetExceeded}
            />
          )}

          {msg.error && (
            <Alert
              type="error"
              showIcon
              style={{ marginTop: 8, borderRadius: 8 }}
              message={t('assistant.error')}
              description={msg.error}
            />
          )}

          {/* Canceled runs are neutral, never a red error: the backend sends
              only done (data.status="canceled") and no error event anymore. */}
          {!msg.streaming && msg.doneStatus === 'canceled' && !msg.error && (
            <Text type="secondary" style={{ fontSize: 12 }}>{t('assistant.runCanceled')}</Text>
          )}
        </div>
      </div>
    );
  };

  return (
    <Drawer
      title={
        <Space>
          <RobotOutlined style={{ color: '#0071e3' }} />
          <span>{t('assistant.title')}</span>
          {activeSession && (
            <Tag color={activeSession.agentType === 'diagnose' ? 'orange' : 'blue'}>
              {activeSession.agentType === 'diagnose' ? t('assistant.diagnose') : t('assistant.copilot')}
            </Tag>
          )}
        </Space>
      }
      open={open}
      onClose={() => setOpen(false)}
      width={760}
      styles={{ body: { padding: 0, display: 'flex', flexDirection: 'column' } }}
    >
      {/* Session toolbar */}
      <div style={{ padding: '10px 16px', borderBottom: '1px solid #f0f0f3', display: 'flex', gap: 8, alignItems: 'center' }}>
        <Select
          size="small"
          style={{ flex: 1 }}
          placeholder={t('assistant.sessions')}
          loading={loadingSessions}
          value={activeKey || undefined}
          onChange={(key) => setActiveKey(key)}
          options={sessions.map((s) => ({
            value: sessionKey(s),
            label: `${s.title || sessionKey(s)} (${s.agentType})`,
          }))}
        />
        <Tooltip title={t('assistant.newSession')}>
          <Button size="small" icon={<PlusOutlined />} loading={creating} onClick={handleNewSession} />
        </Tooltip>
        {activeSession && (
          <Popconfirm
            title={t('assistant.deleteSession.confirm')}
            onConfirm={() => handleDeleteSession(activeSession)}
            okText={t('common.confirm')}
            cancelText={t('common.cancel')}
          >
            <Tooltip title={t('assistant.deleteSession')}>
              <Button size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        )}
        <Tooltip title={t('common.refresh')}>
          <Button size="small" icon={<ReloadOutlined spin={loadingSessions} />} onClick={loadSessions} />
        </Tooltip>
      </div>

      {/* Message stream */}
      <div style={{ flex: 1, overflowY: 'auto', padding: 16, background: '#f5f5f7' }}>
        {messages.length === 0 && (
          <Empty description={t('assistant.empty')} style={{ marginTop: 60 }} />
        )}
        {messages.map(renderMessage)}
        {reconnecting && isRunning && (
          <Alert type="info" showIcon message={t('assistant.reconnecting')} style={{ borderRadius: 8, marginBottom: 8 }} />
        )}
        <div ref={bottomRef} />
      </div>

      {/* Input area */}
      <div style={{ borderTop: '1px solid #f0f0f3', padding: 12, background: '#fff' }}>
        <Space.Compact style={{ width: "100%" }}>
          <Input.TextArea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={t('assistant.placeholder')}
            autoSize={{ minRows: 1, maxRows: 5 }}
            onPressEnter={(e) => {
              if (!e.shiftKey) {
                e.preventDefault();
                handleSend();
              }
            }}
            disabled={!activeSession}
            style={{ borderRadius: '8px 0 0 8px' }}
          />
          {isRunning ? (
            <Button type="default" danger icon={<StopOutlined />} onClick={handleStop} style={{ height: 'auto' }}>
              {t('assistant.stop')}
            </Button>
          ) : (
            <Button
              type="primary"
              icon={<SendOutlined />}
              onClick={handleSend}
              disabled={!activeSession || !input.trim()}
              style={{ height: 'auto' }}
            >
              {t('assistant.send')}
            </Button>
          )}
        </Space.Compact>
      </div>
    </Drawer>
  );
};

export default AssistantDrawer;
