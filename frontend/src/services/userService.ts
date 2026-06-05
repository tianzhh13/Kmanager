import api from './api'

export interface UserStats {
  total: number
  super_admin: number
  cluster_admin: number
  normal_user: number
}

export const userService = {
  getStats: async (): Promise<UserStats> => {
    const response = await api.get('/users/stats')
    return response.data
  },

  list: async (page: number = 1, pageSize: number = 20) => {
    const response = await api.get('/users', {
      params: { page, page_size: pageSize },
    })
    return response.data
  },

  create: async (data: { username: string; password: string; email: string; role: string }) => {
    const response = await api.post('/users', data)
    return response.data
  },

  update: async (userId: number, data: { email?: string; role?: string }) => {
    const response = await api.put(`/users/${userId}`, data)
    return response.data
  },

  delete: async (userId: number) => {
    await api.delete(`/users/${userId}`)
  },

  disable: async (userId: number) => {
    await api.post(`/users/${userId}/disable`)
  },

  enable: async (userId: number) => {
    await api.post(`/users/${userId}/enable`)
  },
}
