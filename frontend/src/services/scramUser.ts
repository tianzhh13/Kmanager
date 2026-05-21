import request from './api'

export interface ScramUser {
  user_id: number
  cluster_id: number
  username: string
  mechanism: string
  sync_status: string
  last_sync_at: string | null
  created_at: string
  updated_at: string
}

export interface CreateScramUserRequest {
  cluster_id: number
  username: string
  password: string
  mechanism?: string
}

export const scramUserService = {
  list: async (clusterId: number, page: number = 1, pageSize: number = 20) => {
    const response = await request.get('/scram-users', {
      params: { cluster_id: clusterId, offset: (page - 1) * pageSize, limit: pageSize },
    })
    return response.data
  },

  create: async (data: CreateScramUserRequest) => {
    const response = await request.post('/scram-users', data)
    return response.data
  },

  delete: async (username: string, clusterId: number) => {
    const response = await request.delete(`/scram-users/${encodeURIComponent(username)}`, {
      params: { cluster_id: clusterId },
    })
    return response.data
  },

  sync: async (clusterId: number) => {
    const response = await request.post(`/scram-users/sync/${clusterId}`)
    return response.data
  },
}
