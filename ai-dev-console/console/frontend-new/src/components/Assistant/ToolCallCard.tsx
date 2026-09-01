import React, { useState } from 'react';
import { Button, Card, Collapse, Space, Tag, Typography } from 'antd';
import {
  CheckOutlined, CloseOutlined, LoadingOutlined, ToolOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { ToolCallInfo } from './types';
import { formatJSON } from './types';

const { Text } = Typography;

interface ToolCallCardProps {
  tool: ToolCallInfo;
  onAnswerConfirmation?: (tool: ToolCallInfo, approved: boolean) => void;
}

const statusTag = (tool: ToolCallInfo, t: (k: string) => string) => {
  switch (tool.status) {
    case 'running':
      return <Tag icon={<LoadingOutlined spin />} color="processing">{t('assistant.tool.running')}</Tag>;
    case 'awaiting_confirmation':
      return <Tag color="warning">{t('assistant.tool.awaiting')}</Tag>;
    case 'succeeded':
      return <Tag color="success">{t('assistant.tool.succeeded')}</Tag>;
    case 'failed':
      return <Tag color="error">{t('assistant.tool.failed')}</Tag>;
    default:
      return null;
  }
};

const ToolCallCard: React.FC<ToolCallCardProps> = ({ tool, onAnswerConfirmation }) => {
  const { t } = useTranslation();
  const [submitting, setSubmitting] = useState<'approve' | 'reject' | null>(null);
  const cfm = tool.confirmation;
  const pendingConfirmation = cfm && !cfm.resolved;

  const answer = async (approved: boolean) => {
    if (!onAnswerConfirmation || submitting) return;
    setSubmitting(approved ? 'approve' : 'reject');
    try {
      await onAnswerConfirmation(tool, approved);
    } finally {
      setSubmitting(null);
    }
  };

  const argsText = tool.input === undefined ? '' : formatJSON(tool.input);
  const resultText = tool.resultText ?? '';

  return (
    <Card
      size="small"
      style={{ borderRadius: 10, borderColor: pendingConfirmation ? '#faad14' : '#f0f0f3', marginTop: 8 }}
      styles={{ body: { padding: '8px 12px' } }}
    >
      <Space size={8} align="center" style={{ width: '100%', justifyContent: 'space-between' }}>
        <Space size={6}>
          <ToolOutlined style={{ color: '#86868b' }} />
          <Text strong style={{ fontSize: 12 }}>{t('assistant.toolCall')}</Text>
          <Text code style={{ fontSize: 12 }}>{tool.name}</Text>
          {statusTag(tool, t)}
        </Space>
      </Space>

      {argsText && (
        <Collapse
          ghost
          size="small"
          style={{ marginTop: 4 }}
          items={[{
            key: 'args',
            label: <Text type="secondary" style={{ fontSize: 12 }}>{t('assistant.tool.arguments')}</Text>,
            children: (
              <pre style={{
                margin: 0, fontSize: 11, lineHeight: 1.5, whiteSpace: 'pre-wrap', wordBreak: 'break-all',
                background: '#f5f5f7', borderRadius: 6, padding: 8, maxHeight: 200, overflow: 'auto',
              }}>
                {argsText}
              </pre>
            ),
          }]}
        />
      )}

      {pendingConfirmation && (
        <div style={{ marginTop: 8, padding: '8px 10px', background: '#fffbe6', borderRadius: 8, border: '1px solid #ffe58f' }}>
          <Text style={{ fontSize: 12 }}>{t('assistant.confirmation.hint')}</Text>
          <div style={{ marginTop: 8 }}>
            <Space>
              <Button
                size="small"
                type="primary"
                icon={<CheckOutlined />}
                loading={submitting === 'approve'}
                disabled={submitting !== null}
                onClick={() => answer(true)}
              >
                {t('assistant.approve')}
              </Button>
              <Button
                size="small"
                danger
                icon={<CloseOutlined />}
                loading={submitting === 'reject'}
                disabled={submitting !== null}
                onClick={() => answer(false)}
              >
                {t('assistant.reject')}
              </Button>
            </Space>
          </div>
        </div>
      )}

      {cfm?.resolved && (
        <div style={{ marginTop: 6 }}>
          {cfm.approved
            ? <Tag color="success">{t('assistant.approved')}</Tag>
            : <Tag color="default">{t('assistant.rejected')}</Tag>}
        </div>
      )}

      {resultText && (
        <Collapse
          ghost
          size="small"
          style={{ marginTop: 4 }}
          items={[{
            key: 'result',
            label: (
              <Text type="secondary" style={{ fontSize: 12 }}>
                {t('assistant.tool.result')}
                {tool.resultState ? ` (${tool.resultState})` : ''}
              </Text>
            ),
            children: (
              <pre style={{
                margin: 0, fontSize: 11, lineHeight: 1.5, whiteSpace: 'pre-wrap', wordBreak: 'break-all',
                background: '#f5f5f7', borderRadius: 6, padding: 8, maxHeight: 240, overflow: 'auto',
              }}>
                {resultText}
              </pre>
            ),
          }]}
        />
      )}
    </Card>
  );
};

export default ToolCallCard;
