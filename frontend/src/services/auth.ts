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

  logout: () => {
    localStorage.removeItem('access_token')
    localStorage.removeItem('refresh_token')
  },
}