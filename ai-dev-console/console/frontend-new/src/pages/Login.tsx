import React, { useState } from 'react';
import { Button, Typography, Input, Divider, message, Tooltip } from 'antd';
import {
  CodeOutlined,
  ThunderboltOutlined,
  RocketOutlined,
  ExperimentOutlined,
  KeyOutlined,
  TranslationOutlined,
  ArrowRightOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { post } from '../api/client';
import { useUserStore } from '../store/user';
import BrandLogo from '../components/BrandLogo';

const { Title, Text } = Typography;

const Login: React.FC = () => {
  const { t } = useTranslation();
  const locale = useUserStore((s) => s.locale);
  const setLocale = useUserStore((s) => s.setLocale);
  const [tokenMode, setTokenMode] = useState(false);
  const [token, setToken] = useState('');
  const [tokenLoading, setTokenLoading] = useState(false);

  const handleLogin = () => {
    window.location.href = '/api/v1/login/aliyun';
  };

  const toggleLocale = () => setLocale(locale === 'zh' ? 'en' : 'zh');

  const handleTokenLogin = async () => {
    if (!token.trim()) {
      message.error(t('login.token.empty'));
      return;
    }
    setTokenLoading(true);
    try {
      await post('/login/token', { token: token.trim() });
      window.location.href = '/';
    } catch {
      message.error(t('login.token.failed'));
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
    <div style={{ minHeight: '100vh', display: 'flex', flexWrap: 'wrap', background: '#fbfbfd' }}>
      {/* Left — cinematic brand scene */}
      <div
        className="login-scene"
        style={{
          flex: '1 1 460px',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          alignItems: 'center',
          padding: '64px 48px',
          minWidth: 320,
        }}
      >
        <div className="aurora aurora-a" />
        <div className="aurora aurora-b" />
        <div className="aurora aurora-c" />
        <div className="grid-overlay" />
        <div className="noise-overlay" />

        <div style={{ position: 'relative', zIndex: 1, maxWidth: 440, width: '100%' }}>
          <div className="anim-fade-up" style={{ marginBottom: 44 }}>
            <BrandLogo size={40} title={t('login.title')} subtitle={t('app.subtitle')} />
          </div>

          <h1
            className="anim-fade-up"
            style={{
              ['--i' as string]: 1,
              margin: '0 0 14px',
              fontSize: 42,
              fontWeight: 700,
              letterSpacing: '-0.03em',
              lineHeight: 1.15,
              color: '#fff',
            }}
          >
            {t('login.subtitle')}
          </h1>

          <p
            className="anim-fade-up"
            style={{ ['--i' as string]: 2, margin: '0 0 44px', fontSize: 15, lineHeight: 1.7, color: 'rgba(255,255,255,0.55)' }}
          >
            {t('login.description')}
          </p>

          <div className="stagger" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {features.map((f, i) => (
              <div
                key={i}
                className="glass-chip"
                style={{
                  ['--i' as string]: i + 3,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 14,
                  padding: '14px 16px',
                  borderRadius: 14,
                }}
              >
                <div
                  style={{
                    width: 36,
                    height: 36,
                    borderRadius: 10,
                    background: 'rgba(255,255,255,0.09)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    color: '#a5b4fc',
                    fontSize: 16,
                    flexShrink: 0,
                  }}
                >
                  {f.icon}
                </div>
                <span style={{ color: 'rgba(255,255,255,0.85)', fontSize: 13.5, letterSpacing: 0.1 }}>{f.text}</span>
              </div>
            ))}
          </div>
        </div>

        <div style={{ position: 'absolute', bottom: 28, left: 0, right: 0, textAlign: 'center', zIndex: 1 }}>
          <span style={{ color: 'rgba(255,255,255,0.28)', fontSize: 12, letterSpacing: 0.3 }}>{t('login.footer')}</span>
        </div>
      </div>

      {/* Right — sign-in form */}
      <div
        style={{
          width: 500,
          maxWidth: '100%',
          flexShrink: 0,
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          alignItems: 'center',
          padding: '64px 56px',
          position: 'relative',
        }}
      >
        <div style={{ position: 'absolute', top: 24, right: 24 }}>
          <Tooltip title={locale === 'zh' ? 'English' : '中文'}>
            <Button
              type="text"
              aria-label="Switch language"
              icon={<TranslationOutlined />}
              onClick={toggleLocale}
              style={{ color: 'var(--text-2)' }}
            />
          </Tooltip>
        </div>

        <div style={{ width: '100%', maxWidth: 340 }}>
          <div className="anim-fade-up">
            <Title level={2} style={{ margin: '0 0 10px', fontWeight: 700, letterSpacing: '-0.02em', color: 'var(--text-1)' }}>
              {t('login.title')}
            </Title>
            <Text style={{ fontSize: 14, color: 'var(--text-2)', display: 'block', marginBottom: 40 }}>
              {t('login.subtitleForm')}
            </Text>
          </div>

          {!tokenMode ? (
            <>
              <div className="anim-fade-up" style={{ ['--i' as string]: 1 }}>
                <Button
                  type="primary"
                  size="large"
                  block
                  onClick={handleLogin}
                  className="pressable"
                  style={{ height: 50, borderRadius: 13, fontSize: 15, fontWeight: 600, letterSpacing: 0.2 }}
                >
                  {t('login.button')}
                  <ArrowRightOutlined style={{ fontSize: 13, marginLeft: 4 }} />
                </Button>
              </div>

              <Divider className="anim-fade-up" style={{ ['--i' as string]: 2, margin: '28px 0', color: 'var(--text-3)', fontSize: 12 }}>
                OR
              </Divider>

              <div className="anim-fade-up" style={{ ['--i' as string]: 3 }}>
                <Button
                  size="large"
                  icon={<KeyOutlined />}
                  onClick={() => setTokenMode(true)}
                  block
                  className="pressable"
                  style={{ height: 50, fontSize: 14, borderRadius: 13, borderColor: 'var(--border-1)' }}
                >
                  {t('login.token.button')}
                </Button>
              </div>
            </>
          ) : (
            <div className="anim-fade-up" style={{ ['--i' as string]: 1 }}>
              <div style={{ marginBottom: 16 }}>
                <Text strong style={{ display: 'block', marginBottom: 8, fontSize: 13 }}>
                  {t('login.token.label')}
                </Text>
                <Input.TextArea
                  rows={5}
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  placeholder={t('login.token.placeholder')}
                  style={{ fontFamily: 'SF Mono, Monaco, Menlo, monospace', fontSize: 12, borderRadius: 10 }}
                />
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                <Button
                  type="primary"
                  size="large"
                  icon={<KeyOutlined />}
                  onClick={handleTokenLogin}
                  loading={tokenLoading}
                  block
                  className="pressable"
                  style={{ height: 50, borderRadius: 13 }}
                >
                  {t('login.token.submit')}
                </Button>
                <Button size="small" type="link" onClick={() => { setTokenMode(false); setToken(''); }} style={{ padding: 0 }}>
                  {t('login.token.back')}
                </Button>
              </div>
              <div
                style={{
                  marginTop: 16,
                  padding: '12px 16px',
                  background: 'var(--bg-canvas)',
                  borderRadius: 10,
                  border: '1px solid var(--border-2)',
                }}
              >
                <Text style={{ fontSize: 11, lineHeight: 1.6, color: 'var(--text-2)' }}>
                  {t('login.token.hint')}
                  <br />
                  <code style={{ fontSize: 10 }}>kubectl get secret &lt;sa-name&gt;-token -o jsonpath=&#123;.data.token&#125; | base64 -d</code>
                </Text>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default Login;
