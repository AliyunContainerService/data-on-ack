import { useEffect, useState } from 'react'
import { lazy, Suspense } from 'react'
import { createBrowserRouter, Navigate } from 'react-router-dom'
import { Spin } from 'antd'
import MainLayout from '@/layouts/MainLayout'

const Login = lazy(() => import('@/pages/Login'))
const Dashboard = lazy(() => import('@/pages/Dashboard'))
const Nodes = lazy(() => import('@/pages/Nodes'))
const QuotaTree = lazy(() => import('@/pages/QuotaTree'))
const Workloads = lazy(() => import('@/pages/Workloads'))
const CostDashboard = lazy(() => import('@/pages/CostDashboard'))
const ModelHub = lazy(() => import('@/pages/ModelHub'))
const ResearcherList = lazy(() => import('@/pages/ResearcherList'))
const ResearcherGroup = lazy(() => import('@/pages/ResearcherGroup'))
const DatasetList = lazy(() => import('@/pages/DatasetList'))
const Images = lazy(() => import('@/pages/Images'))
const Settings = lazy(() => import('@/pages/Settings'))

const PageLoading = (
  <div style={{ height: '60vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
    <Spin size="large" />
  </div>
)

const withSuspense = (element: React.ReactNode) => <Suspense fallback={PageLoading}>{element}</Suspense>
import { useUserStore } from '@/store/user'

function ProtectedLayout() {
  const user = useUserStore((s) => s.user)
  const fetchUserInfo = useUserStore((s) => s.fetchUserInfo)
  // On a hard refresh the store is empty; do not redirect until we know the session state.
  const [initialized, setInitialized] = useState(() => useUserStore.getState().user !== null)

  useEffect(() => {
    if (initialized) return
    let cancelled = false
    // Fetch user info first; only after the result arrives do we decide whether to
    // render the protected routes or redirect to /login (avoids the redirect flash on refresh).
    fetchUserInfo().finally(() => {
      if (!cancelled) setInitialized(true)
    })
    return () => {
      cancelled = true
    }
  }, [fetchUserInfo, initialized])

  if (!initialized) {
    return (
      <div style={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Spin size="large" />
      </div>
    )
  }
  if (!user) {
    return <Navigate to="/login" replace />
  }
  return <MainLayout />
}

export const router = createBrowserRouter([
  { path: '/login', element: withSuspense(<Login />) },
  {
    element: <ProtectedLayout />,
    children: [
      { path: '/', element: <Navigate to="/dashboard" replace /> },
      { path: '/dashboard', element: withSuspense(<Dashboard />) },
      { path: '/nodes', element: withSuspense(<Nodes />) },
      { path: '/quota', element: withSuspense(<QuotaTree />) },
      { path: '/workloads', element: withSuspense(<Workloads />) },
      { path: '/cost', element: withSuspense(<CostDashboard />) },
      { path: '/models', element: withSuspense(<ModelHub />) },
      { path: '/researchers', element: withSuspense(<ResearcherList />) },
      { path: '/user-groups', element: withSuspense(<ResearcherGroup />) },
      { path: '/datasets', element: withSuspense(<DatasetList />) },
      { path: '/images', element: withSuspense(<Images />) },
      { path: '/settings', element: withSuspense(<Settings />) },
    ],
  },
])
