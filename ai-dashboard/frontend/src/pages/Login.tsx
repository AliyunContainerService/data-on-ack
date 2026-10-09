import { useEffect } from 'react'
import { Button, Typography, Tooltip } from 'antd'
import {
  SafetyCertificateOutlined,
  DashboardOutlined,
  ClusterOutlined,
  TranslationOutlined,
  ArrowRightOutlined,
} from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useUserStore } from '@/store/user'
import { changeLanguage } from '@/i18n'
import BrandLogo from '@/components/BrandLogo'

const { Title, Text } = Typography

export default function Login() {
  const { t } = useTranslation()
  const { i18n } = useTranslation()
  const navigate = useNavigate()
  const { fetchUserInfo } = useUserStore()

  useEffect(() => {
    fetchUserInfo().then((ok) => {
      if (ok) navigate('/')
    })
  }, [])

  const handleLogin = () => {
    window.location.href = '/login/aliyun'
  }
  const toggleLocale = () => changeLanguage(i18n.language === 'zh' ? 'en' : 'zh')

  const features = [
    { icon: <ClusterOutlined />, text: t('login.feature1') },
    { icon: <DashboardOutlined />, text: t('login.feature2') },
    { icon: <SafetyCertificateOutlined />, text: t('login.feature3') },
  ]

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
            <BrandLogo size={40} title={t('login.title')} subtitle={t('brandSub')} />
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
            {t('login.heroDesc')}
          </p>

          <div className="stagger" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {features.map((item, idx) => (
              <div
                key={idx}
                className="glass-chip"
                style={{
                  ['--i' as string]: idx + 3,
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
                    color: '#7db8ff',
                    fontSize: 16,
                    flexShrink: 0,
                  }}
                >
                  {item.icon}
                </div>
                <span style={{ color: 'rgba(255,255,255,0.85)', fontSize: 13.5, letterSpacing: 0.1 }}>
                  {item.text}
                </span>
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
          <Tooltip title={i18n.language === 'zh' ? 'English' : '中文'}>
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
            <Text style={{ fontSize: 14, color: 'var(--text-2)', display: 'block', marginBottom: 44 }}>
              {t('login.subtitleForm')}
            </Text>
          </div>

          <div className="anim-fade-up" style={{ ['--i' as string]: 1 }}>
            <Button
              type="primary"
              size="large"
              block
              onClick={handleLogin}
              className="pressable"
              style={{
                height: 50,
                borderRadius: 13,
                fontSize: 15,
                fontWeight: 600,
                letterSpacing: 0.2,
              }}
            >
              {t('login.button')}
              <ArrowRightOutlined style={{ fontSize: 13, marginLeft: 4 }} />
            </Button>
          </div>

          <div
            className="anim-fade-up"
            style={{
              ['--i' as string]: 2,
              marginTop: 24,
              padding: '16px 18px',
              background: 'var(--bg-canvas)',
              borderRadius: 12,
              border: '1px solid var(--border-2)',
            }}
          >
            <Text style={{ fontSize: 12, lineHeight: 1.7, color: 'var(--text-2)' }}>{t('login.ssoNote')}</Text>
          </div>

          <div className="anim-fade-up" style={{ ['--i' as string]: 3, marginTop: 28, textAlign: 'center' }}>
            <span className="status-pill ok live">
              <span className="dot" />
              {t('header.live')}
            </span>
          </div>
        </div>
      </div>
    </div>
  )
}
