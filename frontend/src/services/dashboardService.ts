import api from './api'

export interface DashboardOverview {
  clusters: {
    total: number
    healthy: number
    warning: number
    error: number
    unknown: number
  }
  topics_total: number
  brokers_online: number | null
  partitions_total: number | null
  users_total: number
  consumer_groups: {
    total: number
    total_lag: number
  } | null
  auth_type_distribution: Record<string, number>
  cluster_sizes: Array<{
    cluster_id: number
    cluster_name: string
    broker_count: number | null
    topic_count: number
    health_status: string
  }>
}

export const dashboardService = {
  getOverview: async (): Promise<DashboardOverview> => {
    const response = await api.get('/dashboard/overview')
    return response.data
  },
}
