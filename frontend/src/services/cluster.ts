import api from './api'

export interface Cluster {
  cluster_id: number
  cluster_name: string
  bootstrap_servers: string
  auth_type: string
  sasl_mechanism?: string
  auth_config?: string
  jmx_exporter_urls: string  // 多个 URL 逗号分隔
  description: string
  status: string
  created_at: string
  updated_at: string
}

export interface CreateClusterRequest {
  cluster_name: string
  bootstrap_servers: string
  auth_type: string
  auth_config?: Record<string, any>
  jmx_exporter_urls?: string  // 多个 URL 逗号分隔
  description?: string
}

export interface UpdateClusterRequest {
  cluster_name?: string
  bootstrap_servers?: string
  auth_type?: string
  auth_config?: Record<string, any>
  jmx_exporter_urls?: string  // 多个 URL 逗号分隔
  description?: string
  status?: string
}

export interface ClusterWithStats extends Cluster {
  broker_count?: number
  topic_count?: number
  health_status?: string
}

export interface ClusterListResponse {
  data: ClusterWithStats[]
  total: number
  page: number
  page_size: number
}

// 集群基础信息（工单申请选择集群用，所有登录用户可见，不含连接与认证细节）
export interface ClusterOption {
  cluster_id: number
  cluster_name: string
  auth_type: string
  sasl_mechanism?: string
  status: string
}

export const clusterAPI = {
  // 集群基础信息（不做权限过滤，所有登录用户可见）
  options: async (): Promise<ClusterOption[]> => {
    const response = await api.get('/clusters/options')
    return response.data?.data || []
  },

  list: async (page: number = 1, pageSize: number = 20, withStats?: boolean) => {
    const response = await api.get('/clusters', {
      params: { page, page_size: pageSize, with_stats: withStats || undefined },
    })
    return response.data
  },

  listWithStats: async (page: number = 1, pageSize: number = 100): Promise<ClusterListResponse> => {
    const response = await api.get('/clusters', {
      params: { page, page_size: pageSize, with_stats: true },
    })
    return response.data
  },

  get: async (id: number): Promise<Cluster> => {
    const response = await api.get(`/clusters/${id}`)
    return response.data
  },

  create: async (data: CreateClusterRequest): Promise<Cluster> => {
    const response = await api.post('/clusters', data)
    return response.data
  },

  update: async (id: number, data: UpdateClusterRequest): Promise<Cluster> => {
    const response = await api.put(`/clusters/${id}`, data)
    return response.data
  },

  delete: async (id: number): Promise<void> => {
    await api.delete(`/clusters/${id}`)
  },

  testConnection: async (id: number): Promise<void> => {
    await api.post(`/clusters/${id}/test`)
  },

  testConnectionForCreate: async (data: CreateClusterRequest): Promise<void> => {
    await api.post('/clusters/test-connection', data)
  },

  uploadKeytab: async (file: File): Promise<string> => {
    const formData = new FormData()
    formData.append('keytab', file)
    const response = await api.post('/clusters/upload-keytab', formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
    })
    return response.data.temp_id
  },

  deleteTempKeytab: async (tempId: string): Promise<void> => {
    await api.delete('/clusters/upload-keytab', { params: { temp_id: tempId } })
  },

  grantAccess: async (id: number, userId: number): Promise<void> => {
    await api.post(`/clusters/${id}/grant`, { user_id: userId })
  },

  revokeAccess: async (id: number, userId: number): Promise<void> => {
    await api.post(`/clusters/${id}/revoke`, { user_id: userId })
  },

  listUsers: async (id: number) => {
    const response = await api.get(`/clusters/${id}/users`)
    return response.data
  },
}