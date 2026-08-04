import { createBrowserRouter, Navigate } from 'react-router-dom'
import MainLayout from '@/layouts/MainLayout'
import Login from '@/pages/Login'
import Dashboard from '@/pages/Dashboard'
import Nodes from '@/pages/Nodes'
import QuotaTree from '@/pages/QuotaTree'
import Workloads from '@/pages/Workloads'
import CostDashboard from '@/pages/CostDashboard'
import ModelHub from '@/pages/ModelHub'
import ResearcherList from '@/pages/ResearcherList'
import ResearcherGroup from '@/pages/ResearcherGroup'
import DatasetList from '@/pages/DatasetList'
import Images from '@/pages/Images'
import Settings from '@/pages/Settings'
import { useUserStore } from '@/store/user'

function ProtectedLayout() {
  const user = useUserStore((s) => s.user)
  if (!user) {
    return <Navigate to="/login" replace />
  }
  return <MainLayout />
}

export const router = createBrowserRouter([
  { path: '/login', element: <Login /> },
  {
    element: <ProtectedLayout />,
    children: [
      { path: '/', element: <Navigate to="/dashboard" replace /> },
      { path: '/dashboard', element: <Dashboard /> },
      { path: '/nodes', element: <Nodes /> },
      { path: '/quota', element: <QuotaTree /> },
      { path: '/workloads', element: <Workloads /> },
      { path: '/cost', element: <CostDashboard /> },
      { path: '/models', element: <ModelHub /> },
      { path: '/researchers', element: <ResearcherList /> },
      { path: '/user-groups', element: <ResearcherGroup /> },
      { path: '/datasets', element: <DatasetList /> },
      { path: '/images', element: <Images /> },
      { path: '/settings', element: <Settings /> },
    ],
  },
])
