import api from './api'

export interface LoginRequest {
  username: string
  password: string
}

export interface User {
  user_id: number
  username: string
  email: string
  role: string
}

export interface LoginResponse {
  access_token: string
  refresh_token: string
  expires_in: number
  user_info: User
}

export const authService = {
  login: async (data: LoginRequest): Promise<LoginResponse> => {
    const response = await api.post('/auth/login', data)
    return response.data
  },

  refreshToken: async (refreshToken: string): Promise<{ access_token: string }> => {
    const response = await api.post('/auth/refresh', { refresh_token: refreshToken })
    return response.data
  },

  getCurrentUser: async (): Promise<User> => {
    const response = await api.get('/auth/me')
    return response.data
  },

  logout: async () => {
    try {
      // 调用后端退出登录 API，将 Token 加入黑名单
      await api.post('/auth/logout')
    } catch (error) {
      // 即使后端调用失败，也要清除本地存储
      console.error('Logout API failed:', error)
    } finally {
      localStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
    }
  },
}