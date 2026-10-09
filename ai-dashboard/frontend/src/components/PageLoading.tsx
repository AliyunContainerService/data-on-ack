import BrandLogo from './BrandLogo'

/** Branded full-area loading state used for route suspense fallbacks. */
export default function PageLoading({ fullScreen = false }: { fullScreen?: boolean }) {
  return (
    <div
      style={{
        height: fullScreen ? '100vh' : '62vh',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 18,
      }}
      role="status"
      aria-live="polite"
    >
      <div className="anim-scale-in" style={{ animation: 'float-y 2.4s ease-in-out infinite' }}>
        <BrandLogo size={44} />
      </div>
      <div
        className="skeleton-block"
        style={{ width: 120, height: 4, borderRadius: 4 }}
        aria-hidden="true"
      />
    </div>
  )
}
