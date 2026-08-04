import React, { useState } from 'react';
import { Button, Typography, Space, Input, Divider, message } from 'antd';
import { CloudOutlined, CodeOutlined, ThunderboltOutlined, RocketOutlined, ExperimentOutlined, KeyOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { post } from '../api/client';

const { Title, Text, Paragraph } = Typography;

const Login: React.FC = () => {
  const { t } = useTranslation();
  const [tokenMode, setTokenMode] = useState(false);
  const [token, setToken] = useState('');
  const [tokenLoading, setTokenLoading] = useState(false);

  const handleLogin = () => {
    window.location.href = '/api/v1/login/aliyun';
  };

  const handleTokenLogin = async () => {
    if (!token.trim()) {
      message.error('Please enter a valid token');
      return;
    }
    setTokenLoading(true);
    try {
      await post('/login/token', { token: token.trim() });
      window.location.href = '/';
    } catch {
      message.error('Token authentication failed');
    } finally {
      setTokenLoading(false);
    }
  };

  const features = [
    { icon: <CodeOutlined />, text: t('login.feature1') },
    { icon: <ThunderboltOutlined />, text: t('login.feature2') },
    { icon: <RocketOutlined />, text: t('login.feature3') },
    { icon: <ExperimentOutlined />, text: t('login.feature4') },
  ];

  return (
    <div style={{ height: '100vh', display: 'flex' }}>
      {/* Left panel - branding */}
      <div style={{
        flex: 1,
        background: 'linear-gradient(160deg, #0f172a 0%, #1e293b 50%, #1e1b4b 100%)',
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        padding: '60px 80px',
        position: 'relative',
        overflow: 'hidden',
      }}>
        {/* Background decoration */}
        <div style={{
          position: 'absolute', top: -100, right: -100,
          width: 400, height: 400, borderRadius: '50%',
          background: 'radial-gradient(circle, rgba(99,102,241,0.15) 0%, transparent 70%)',
        }} />
        <div style={{
          position: 'absolute', bottom: -50, left: -50,
          width: 300, height: 300, borderRadius: '50%',
          background: 'radial-gradient(circle, rgba(22,119,255,0.1) 0%, transparent 70%)',
        }} />

        <div style={{ position: 'relative', zIndex: 1 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 40 }}>
            <div style={{
              width: 44, height: 44, borderRadius: 10,
              background: 'linear-gradient(135deg, #1677ff, #6366f1)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>
              <ThunderboltOutlined style={{ color: '#fff', fontSize: 22 }} />
            </div>
            <Title level={3} style={{ color: '#fff', margin: 0 }}>{t('app.title')}</Title>
          </div>

          <Title level={1} style={{ color: '#fff', fontWeight: 700, marginBottom: 16, fontSize: 36 }}>
            {t('login.subtitle')}
          </Title>

          <Paragraph style={{ color: 'rgba(255,255,255,0.6)', fontSize: 16, marginBottom: 48, maxWidth: 450 }}>
            Kubernetes-native AI platform with GPU scheduling, elastic quotas, and seamless integration with the cloud-native ecosystem.
          </Paragraph>

          <Space direction="vertical" size={16}>
            {features.map((f, i) => (
              <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <div style={{
                  width: 36, height: 36, borderRadius: 8,
                  background: 'rgba(255,255,255,0.08)',
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  color: '#818cf8', fontSize: 16,
                }}>
                  {f.icon}
                </div>
                <Text style={{ color: 'rgba(255,255,255,0.85)', fontSize: 15 }}>{f.text}</Text>
              </div>
            ))}
          </Space>
        </div>
      </div>

      {/* Right panel - login form */}
      <div style={{
        width: 480,
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        alignItems: 'center',
        padding: '60px',
        background: '#fff',
      }}>
        <div style={{ width: '100%', maxWidth: 360 }}>
          <Title level={2} style={{ marginBottom: 8 }}>
            {t('login.title')}
          </Title>
          <Text type="secondary" style={{ display: 'block', marginBottom: 40 }}>
            Sign in to access your AI development workspace
          </Text>

          {!tokenMode ? (
            <>
              <Button
                type="primary"
                size="large"
                icon={<CloudOutlined />}
                onClick={handleLogin}
                block
                style={{
                  height: 52,
                  fontSize: 16,
                  borderRadius: 10,
                  boxShadow: '0 4px 14px rgba(22,119,255,0.25)',
                }}
              >
                {t('login.button')}
              </Button>

              <Divider style={{ margin: '28px 0', color: '#ccc', fontSize: 12 }}>OR</Divider>

              <Button
                size="large"
                icon={<KeyOutlined />}
                onClick={() => setTokenMode(true)}
                block
                style={{
                  height: 48,
                  fontSize: 14,
                  borderRadius: 10,
                  borderColor: '#e8e8ed',
                }}
              >
                Sign in with Token
              </Button>
            </>
          ) : (
            <>
              <div style={{ marginBottom: 16 }}>
                <Text strong style={{ display: 'block', marginBottom: 8, fontSize: 13 }}>
                  Bearer Token / ServiceAccount Token
                </Text>
                <Input.TextArea
                  rows={5}
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  placeholder="Paste your Kubernetes Bearer Token here..."
                  style={{ fontFamily: 'SF Mono, Monaco, Menlo, monospace', fontSize: 12, borderRadius: 8 }}
                />
              </div>
              <Space style={{ width: '100%' }} direction="vertical" size={12}>
                <Button
                  type="primary"
                  size="large"
                  icon={<KeyOutlined />}
                  onClick={handleTokenLogin}
                  loading={tokenLoading}
                  block
                  style={{ height: 48, borderRadius: 10 }}
                >
                  Authenticate
                </Button>
                <Button
                  size="small"
                  type="link"
                  onClick={() => { setTokenMode(false); setToken(''); }}
                  style={{ padding: 0 }}
                >
                  Back to other login methods
                </Button>
              </Space>
              <div style={{ marginTop: 16, padding: '12px 16px', background: '#f5f5f7', borderRadius: 8 }}>
                <Text type="secondary" style={{ fontSize: 11, lineHeight: 1.5 }}>
                  You can obtain a token from your cluster admin or by running:<br />
                  <code style={{ fontSize: 10 }}>kubectl get secret &lt;sa-name&gt;-token -o jsonpath=&#123;.data.token&#125; | base64 -d</code>
                </Text>
              </div>
            </>
          )}

          <div style={{ marginTop: 32, textAlign: 'center' }}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              Powered by Alibaba Cloud ACK
            </Text>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Login;
