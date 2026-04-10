import { createSlice, createAsyncThunk, PayloadAction } from '@reduxjs/toolkit'
import { authService, LoginRequest, LoginResponse } from '../../services/auth'

interface User {
  user_id: number
  username: string
  email: string
  role: string
}

interface AuthState {
  isAuthenticated: boolean
  user: User | null
  accessToken: string | null
  loading: boolean
  error: string | null
}

const initialState: AuthState = {
  isAuthenticated: !!localStorage.getItem('access_token'),
  user: null,
  accessToken: localStorage.getItem('access_token'),
  loading: false,
  error: null,
}

export const login = createAsyncThunk<LoginResponse, LoginRequest>(
  'auth/login',
  async (credentials, { rejectWithValue }) => {
    try {
      const response = await authService.login(credentials)
      localStorage.setItem('access_token', response.access_token)
      localStorage.setItem('refresh_token', response.refresh_token)
      return response
    } catch (error: any) {
      return rejectWithValue(error.response?.data?.error || '登录失败')
    }
  }
)

export const refreshToken = createAsyncThunk(
  'auth/refreshToken',
  async (_, { rejectWithValue }) => {
    try {
      const refresh_token = localStorage.getItem('refresh_token')
      if (!refresh_token) {
        throw new Error('No refresh token')
      }
      const response = await authService.refreshToken(refresh_token)
      localStorage.setItem('access_token', response.access_token)
      return response
    } catch (error: any) {
      localStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
      return rejectWithValue('Token 刷新失败，请重新登录')
    }
  }
)

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {
    logout: (state) => {
      state.isAuthenticated = false
      state.user = null
      state.accessToken = null
      localStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
    },
    clearError: (state) => {
      state.error = null
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(login.pending, (state) => {
        state.loading = true
        state.error = null
      })
      .addCase(login.fulfilled, (state, action: PayloadAction<LoginResponse>) => {
        state.loading = false
        state.isAuthenticated = true
        state.accessToken = action.payload.access_token
        state.user = action.payload.user_info
      })
      .addCase(login.rejected, (state, action) => {
        state.loading = false
        state.error = action.payload as string
      })
      .addCase(refreshToken.fulfilled, (state, action) => {
        state.accessToken = action.payload.access_token
      })
      .addCase(refreshToken.rejected, (state) => {
        state.isAuthenticated = false
        state.user = null
        state.accessToken = null
      })
  },
})

export const { logout, clearError } = authSlice.actions
export default authSlice.reducer