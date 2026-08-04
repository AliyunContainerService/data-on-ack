import { createBrowserRouter, Navigate } from 'react-router-dom';
import MainLayout from '../layouts/MainLayout';
import Dashboard from '../pages/Dashboard';
import Notebooks from '../pages/Notebooks';
import Training from '../pages/Training';
import FineTune from '../pages/FineTune';
import Serving from '../pages/Serving';
import Models from '../pages/Models';
import Datasets from '../pages/Datasets';
import Experiments from '../pages/Experiments';
import Login from '../pages/Login';

export const router = createBrowserRouter([
  {
    path: '/login',
    element: <Login />,
  },
  {
    path: '/',
    element: <MainLayout />,
    children: [
      { index: true, element: <Dashboard /> },
      { path: 'notebooks', element: <Notebooks /> },
      { path: 'training', element: <Training /> },
      { path: 'finetune', element: <FineTune /> },
      { path: 'experiments', element: <Experiments /> },
      { path: 'serving', element: <Serving /> },
      { path: 'models', element: <Models /> },
      { path: 'datasets', element: <Datasets /> },
    ],
  },
  { path: '*', element: <Navigate to="/" replace /> },
]);
