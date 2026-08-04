import { get } from './client';

export interface DashboardOverview {
  notebooks: ResourceCount;
  trainingJobs: ResourceCount;
  servingJobs: ResourceCount;
}

interface ResourceCount {
  total: number;
  running: number;
  failed: number;
  pending: number;
}

export function getOverview(): Promise<DashboardOverview> {
  return get<DashboardOverview>('/overview');
}
