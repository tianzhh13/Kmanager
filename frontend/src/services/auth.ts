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

  refreshToken: async (): Promise<{ access_token: string }> => {
    // refresh_token 通过 httpOnly Cookie 自动发送
    const response = await api.post('/auth/refresh')
    return response.data
  },

  getCurrentUser: async (): Promise<User> => {
    const response = await api.get('/auth/me')
    return response.data
  },

  logout: async () => {
    try {
      // 调用后端退出登录 API，清除 Cookie 并将 Token 加入黑名单
      await api.post('/auth/logout')
    } catch (error) {
      console.error('Logout API failed:', error)
    }
    // Token 存储在 httpOnly Cookie 中，后端已清除，无需前端操作
  },
}
