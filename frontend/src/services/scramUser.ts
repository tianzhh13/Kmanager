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
    console.log('=== DEBUG: scramUserService.list called, clusterId ===', clusterId)
    const response = await request.get('/scram-users', {
      params: { cluster_id: clusterId, offset: (page - 1) * pageSize, limit: pageSize },
    })
    console.log('=== DEBUG: scramUserService.list response ===', response)
    return response.data
  },

  create: async (data: CreateScramUserRequest) => {
    console.log('=== DEBUG: scramUserService.create called with ===', data)
    const response = await request.post('/scram-users', data)
    console.log('=== DEBUG: scramUserService.create response ===', response)
    return response.data
  },

  delete: async (username: string, clusterId: number) => {
    console.log('=== DEBUG: scramUserService.delete called, username ===', username, 'clusterId ===', clusterId)
    const response = await request.delete(`/scram-users/${encodeURIComponent(username)}`, {
      params: { cluster_id: clusterId },
    })
    console.log('=== DEBUG: scramUserService.delete response ===', response)
    return response.data
  },

  sync: async (clusterId: number) => {
    console.log('=== DEBUG: scramUserService.sync called, clusterId ===', clusterId)
    const response = await request.post(`/scram-users/sync/${clusterId}`)
    console.log('=== DEBUG: scramUserService.sync response ===', response)
    return response.data
  },
}
