import { useEffect } from 'react'
import { Button, Typography } from 'antd'
import { CloudServerOutlined, SafetyCertificateOutlined, DashboardOutlined, ClusterOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useUserStore } from '@/store/user'

const { Title, Text } = Typography

export default function Login() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { fetchUserInfo } = useUserStore()

  useEffect(() => {
    fetchUserInfo().then((ok) => { if (ok) navigate('/') })
  }, [])

  const handleLogin = () => { window.location.href = '/login/aliyun' }

  return (
    <div style={{
      minHeight: '100vh',
      display: 'flex',
      background: '#fbfbfd',
    }}>
      {/* Left panel - branding */}
      <div style={{
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        alignItems: 'center',
        padding: '60px 40px',
        background: 'linear-gradient(160deg, #0a0a0f 0%, #1a1a2e 50%, #16213e 100%)',
        position: 'relative',
        overflow: 'hidden',
      }}>
        {/* Subtle grid pattern */}
        <div style={{
          position: 'absolute', inset: 0, opacity: 0.03,
          backgroundImage: 'radial-gradient(circle, #fff 1px, transparent 1px)',
          backgroundSize: '32px 32px',
        }} />
        {/* Glow effects */}
        <div style={{ position: 'absolute', top: '20%', left: '30%', width: 300, height: 300, borderRadius: '50%', background: 'radial-gradient(circle, rgba(0,113,227,0.15) 0%, transparent 70%)' }} />
        <div style={{ position: 'absolute', bottom: '20%', right: '20%', width: 200, height: 200, borderRadius: '50%', background: 'radial-gradient(circle, rgba(114,46,209,0.12) 0%, transparent 70%)' }} />

        <div style={{ position: 'relative', zIndex: 1, maxWidth: 420, textAlign: 'center' }}>
          <div style={{
            width: 72, height: 72, borderRadius: 20, margin: '0 auto 28px',
            background: 'linear-gradient(135deg, #0071e3, #5856d6)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            boxShadow: '0 8px 32px rgba(0,113,227,0.3)',
          }}>
            <CloudServerOutlined style={{ fontSize: 36, color: '#fff' }} />
          </div>
          <Title level={2} style={{ color: '#fff', margin: '0 0 8px', fontWeight: 600, letterSpacing: '-0.02em' }}>
            AI Suite Operations
          </Title>
          <Text style={{ color: 'rgba(255,255,255,0.6)', fontSize: 15 }}>
            {t('login.subtitle')}
          </Text>

          {/* Feature highlights */}
          <div style={{ marginTop: 48, textAlign: 'left', display: 'flex', flexDirection: 'column', gap: 16 }}>
            {[
              { icon: <ClusterOutlined />, text: 'GPU Cluster Management & Monitoring' },
              { icon: <DashboardOutlined />, text: 'Resource Quota & Cost Analytics' },
              { icon: <SafetyCertificateOutlined />, text: 'Multi-tenant RBAC & Security' },
            ].map((item, idx) => (
              <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <div style={{
                  width: 36, height: 36, borderRadius: 10,
                  background: 'rgba(255,255,255,0.06)', border: '1px solid rgba(255,255,255,0.08)',
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  color: 'rgba(255,255,255,0.7)', fontSize: 16,
                }}>
                  {item.icon}
                </div>
                <Text style={{ color: 'rgba(255,255,255,0.7)', fontSize: 13 }}>{item.text}</Text>
              </div>
            ))}
          </div>
        </div>

        {/* Footer */}
        <div style={{ position: 'absolute', bottom: 32, left: 0, right: 0, textAlign: 'center' }}>
          <Text style={{ color: 'rgba(255,255,255,0.3)', fontSize: 12 }}>
            Powered by Alibaba Cloud ACK
          </Text>
        </div>
      </div>

      {/* Right panel - login form */}
      <div style={{
        width: 480,
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        alignItems: 'center',
        padding: '60px 56px',
      }}>
        <div style={{ width: '100%', maxWidth: 340 }}>
          <Title level={3} style={{ margin: '0 0 8px', fontWeight: 600, color: '#1d1d1f' }}>
            {t('login.title')}
          </Title>
          <Text type="secondary" style={{ fontSize: 14, display: 'block', marginBottom: 40 }}>
            Sign in to access the operations console
          </Text>

          <Button
            type="primary"
            size="large"
            block
            onClick={handleLogin}
            style={{
              height: 48,
              borderRadius: 12,
              fontSize: 15,
              fontWeight: 500,
              background: '#0071e3',
              border: 'none',
              boxShadow: '0 4px 12px rgba(0,113,227,0.25)',
            }}
          >
            {t('login.button')}
          </Button>

          <div style={{ marginTop: 24, padding: '16px 20px', background: '#f5f5f7', borderRadius: 12 }}>
            <Text type="secondary" style={{ fontSize: 12, lineHeight: 1.6 }}>
              You will be redirected to Alibaba Cloud RAM SSO for authentication. Only authorized accounts can access this console.
            </Text>
          </div>
        </div>
      </div>
    </div>
  )
}
