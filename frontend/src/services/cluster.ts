import api from './api'

export interface Cluster {
  cluster_id: number
  cluster_name: string
  bootstrap_servers: string
  auth_type: string
  auth_config?: string
  prometheus_url: string
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
  prometheus_url?: string
  description?: string
}

export interface UpdateClusterRequest {
  cluster_name?: string
  bootstrap_servers?: string
  auth_type?: string
  auth_config?: Record<string, any>
  prometheus_url?: string
  description?: string
  status?: string
}

export const clusterAPI = {
  list: async (page: number = 1, pageSize: number = 20) => {
    const response = await api.get('/clusters', {
      params: { page, page_size: pageSize },
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