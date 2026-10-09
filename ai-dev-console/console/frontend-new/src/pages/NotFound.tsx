import React from 'react';
import { Button } from 'antd';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import BrandLogo from '../components/BrandLogo';

const NotFound: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();

  return (
    <div
      style={{
        minHeight: 'calc(100vh - 108px)',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 8,
        textAlign: 'center',
      }}
    >
      <div className="anim-scale-in" style={{ marginBottom: 20 }}>
        <BrandLogo size={56} />
      </div>
      <div
        className="text-gradient anim-fade-up"
        style={{ fontSize: 88, fontWeight: 800, letterSpacing: '-0.04em', lineHeight: 1 }}
      >
        404
      </div>
      <h2 className="anim-fade-up" style={{ ['--i' as string]: 1, margin: '12px 0 4px', fontSize: 20, color: 'var(--text-1)' }}>
        {t('notFound.title')}
      </h2>
      <p className="anim-fade-up" style={{ ['--i' as string]: 2, margin: 0, color: 'var(--text-2)', fontSize: 13 }}>
        {t('notFound.desc')}
      </p>
      <Button
        type="primary"
        className="anim-fade-up"
        style={{ ['--i' as string]: 3, marginTop: 24 }}
        onClick={() => navigate('/')}
      >
        {t('notFound.back')}
      </Button>
    </div>
  );
};

export default NotFound;
