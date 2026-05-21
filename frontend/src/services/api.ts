import axios from 'axios'

// 简单的baseURL配置
// 开发环境：Vite代理到 localhost:18080
// 生产环境：前端和后端在同一域名下，使用相对路径
const baseURL = import.meta.env.DEV ? '/api/v1' : '/api/v1'

const api = axios.create({
  baseURL,
  timeout: 30000,
  withCredentials: true, // 跨域时携带 Cookie
})

// Token刷新Promise单例，用于防止并发刷新请求
// 当多个请求同时收到401时，共享同一个刷新Promise，避免重复刷新
let refreshPromise: Promise<void> | null = null

// 请求拦截器 - 添加 CSRF 保护头
// Token 通过 httpOnly Cookie 自动发送，无需手动添加 Authorization
api.interceptors.request.use(
  (config) => {
    // CSRF 保护：X-Requested-With 头
    // 配合 SameSite=Strict/Lax Cookie 使用，可防止 CSRF 攻击
    // 后端应验证此头存在且值为 XMLHttpRequest
    config.headers['X-Requested-With'] = 'XMLHttpRequest'
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器 - 处理错误
api.interceptors.response.use(
  (response) => {
    return response
  },
  async (error) => {
    const originalRequest = error.config

    // 如果是 401 且没有重试过，尝试刷新 Token
    // 但 /auth/me 和 /auth/refresh 本身返回 401 不应触发刷新（避免死循环）
    if (error.response?.status === 401 && !originalRequest._retry) {
      const url = originalRequest.url || ''
      const isAuthEndpoint = url.includes('/auth/me') || url.includes('/auth/refresh')

      if (!isAuthEndpoint) {
        originalRequest._retry = true

        // 如果已有刷新请求在进行中，等待它完成
        if (refreshPromise) {
          try {
            await refreshPromise
            return api(originalRequest)
          } catch {
            return Promise.reject(error)
          }
        }

        // 刷新 Token（refresh_token 在 Cookie 中，后端自动读取）
        refreshPromise = axios.post('/api/v1/auth/refresh', {}, {
          withCredentials: true,
        }).then(() => {
          // 刷新成功
        }).finally(() => {
          // 清除Promise引用，允许后续刷新
          refreshPromise = null
        })

        try {
          await refreshPromise
          // 刷新成功，新 access_token 已通过 Cookie 设置，重试原请求
          return api(originalRequest)
        } catch {
          // 刷新失败，使用 history 替换当前 URL（不触发全页刷新）
          if (window.location.pathname !== '/login') {
            window.history.replaceState(null, '', '/login')
          }
          return Promise.reject(error)
        }
      }
    }

    return Promise.reject(error)
  }
)

export default api
