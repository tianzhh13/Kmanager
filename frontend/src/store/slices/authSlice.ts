import { createSlice, createAsyncThunk, PayloadAction } from '@reduxjs/toolkit'
import { authService, LoginRequest, User as AuthUser } from '../../services/auth'

interface AuthState {
  isAuthenticated: boolean
  user: AuthUser | null
  loading: boolean
  error: string | null
  initialized: boolean
}

const initialState: AuthState = {
  isAuthenticated: false,
  user: null,
  loading: false,
  error: null,
  initialized: false,
}

// 本地 User 类型（与 auth.ts 中的 User 对齐，避免引入 LoginResponse）
export const login = createAsyncThunk<AuthUser, LoginRequest>(
  'auth/login',
  async (credentials, { rejectWithValue }) => {
    try {
      const response = await authService.login(credentials)
      return response.user_info // 只存储用户信息，不存储 token
    } catch (error: any) {
      return rejectWithValue(error.response?.data?.error || '登录失败')
    }
  }
)

export const checkAuth = createAsyncThunk<AuthUser>(
  'auth/checkAuth',
  async (_, { rejectWithValue }) => {
    try {
      return await authService.getCurrentUser()
    } catch (error: any) {
      return rejectWithValue('认证已过期')
    }
  }
)

export const logoutAsync = createAsyncThunk(
  'auth/logoutAsync',
  async () => {
    await authService.logout()
  }
)

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {
    logout: (state) => {
      state.isAuthenticated = false
      state.user = null
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
      .addCase(login.fulfilled, (state, action: PayloadAction<AuthUser>) => {
        state.loading = false
        state.isAuthenticated = true
        state.user = action.payload
        state.error = null
      })
      .addCase(login.rejected, (state, action) => {
        state.loading = false
        state.error = action.payload as string
      })
      .addCase(checkAuth.fulfilled, (state, action: PayloadAction<AuthUser>) => {
        state.isAuthenticated = true
        state.user = action.payload
        state.initialized = true
      })
      .addCase(checkAuth.rejected, (state) => {
        state.isAuthenticated = false
        state.user = null
        state.initialized = true
      })
      .addCase(logoutAsync.fulfilled, (state) => {
        state.isAuthenticated = false
        state.user = null
      })
  },
})

export const { logout, clearError } = authSlice.actions
export default authSlice.reducer
