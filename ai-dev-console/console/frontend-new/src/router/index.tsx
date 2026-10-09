import { lazy, Suspense } from 'react';
import { createBrowserRouter } from 'react-router-dom';
import MainLayout from '../layouts/MainLayout';
import PageLoading from '../components/PageLoading';

const Login = lazy(() => import('../pages/Login'));
const Dashboard = lazy(() => import('../pages/Dashboard'));
const Notebooks = lazy(() => import('../pages/Notebooks'));
const Training = lazy(() => import('../pages/Training'));
const FineTune = lazy(() => import('../pages/FineTune'));
const Serving = lazy(() => import('../pages/Serving'));
const Models = lazy(() => import('../pages/Models'));
const Datasets = lazy(() => import('../pages/Datasets'));
const Experiments = lazy(() => import('../pages/Experiments'));
const NotFound = lazy(() => import('../pages/NotFound'));

const withSuspense = (element: React.ReactNode) => <Suspense fallback={<PageLoading />}>{element}</Suspense>;

export const router = createBrowserRouter([
  {
    path: '/login',
    element: withSuspense(<Login />),
  },
  {
    path: '/',
    element: <MainLayout />,
    children: [
      { index: true, element: withSuspense(<Dashboard />) },
      { path: 'notebooks', element: withSuspense(<Notebooks />) },
      { path: 'training', element: withSuspense(<Training />) },
      { path: 'finetune', element: withSuspense(<FineTune />) },
      { path: 'experiments', element: withSuspense(<Experiments />) },
      { path: 'serving', element: withSuspense(<Serving />) },
      { path: 'models', element: withSuspense(<Models />) },
      { path: 'datasets', element: withSuspense(<Datasets />) },
      { path: '*', element: withSuspense(<NotFound />) },
    ],
  },
]);
