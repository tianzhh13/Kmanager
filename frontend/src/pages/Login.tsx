import { useState, useCallback } from 'react'
import { message } from 'antd'
import { useNavigate } from 'react-router-dom'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { login, clearError } from '../store/slices/authSlice'
import './LoginV2.css'

const Login: React.FC = () => {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [remember, setRemember] = useState(false)
  const navigate = useNavigate()
  const dispatch = useAppDispatch()
  const { loading } = useAppSelector((state) => state.auth)

  const handleSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault()
    if (!username || !password) return
    dispatch(clearError())
    const result = await dispatch(login({ username, password }))
    if (login.fulfilled.match(result)) {
      message.success('登录成功')
      navigate('/dashboard')
    } else {
      message.error(result.payload as string)
    }
  }, [username, password, dispatch, navigate])

  return (
    <div className="login-page-v2">
      {/* Floating orbs */}
      <div className="login-orb login-orb-1" />
      <div className="login-orb login-orb-2" />
      <div className="login-orb login-orb-3" />
      <div className="login-noise" />

      {/* Left brand column */}
      <div className="login-left">
        <div className="login-eyebrow">Enterprise Platform</div>
        <h1 className="login-h1">Kmanager</h1>
        <p className="login-sub">
          企业级 Kafka 集群管理平台，统一管控多集群配置、认证、监控与权限。
        </p>
        <div className="login-stats">
          <div className="login-stat">
            <span className="login-stat-val">50+</span>
            <span className="login-stat-lbl">Clusters</span>
          </div>
          <div className="login-stat">
            <span className="login-stat-val">24/7</span>
            <span className="login-stat-lbl">Monitoring</span>
          </div>
          <div className="login-stat">
            <span className="login-stat-val">RBAC</span>
            <span className="login-stat-lbl">Security</span>
          </div>
        </div>
      </div>

      {/* Right login column */}
      <div className="login-right">
        {/* Ambient card (Z-Axis Cascade) */}
        <div className="login-card-ambient" />

        {/* Main Double-Bezel card */}
        <div className="login-card-shell">
          <div className="login-card-core">
            {/* Brand icon */}
            <div className="login-brand-v2">
              <div className="login-brand-icon-v2">
                <span>Km</span>
              </div>
              <div>
                <div className="login-brand-name">Kmanager</div>
                <div className="login-brand-ver">v2.0</div>
              </div>
            </div>

            {/* Form */}
            <form className="login-form-v2" onSubmit={handleSubmit}>
              <div className="login-field-v2">
                <label className="login-field-label">用户名</label>
                <div className="login-field-shell">
                  <div className="login-field-core">
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
                      <circle cx="12" cy="7" r="4" />
                    </svg>
                    <input
                      type="text"
                      value={username}
                      onChange={e => setUsername(e.target.value)}
                      placeholder="请输入用户名"
                      autoComplete="username"
                      autoFocus
                    />
                  </div>
                </div>
              </div>

              <div className="login-field-v2">
                <label className="login-field-label">密码</label>
                <div className="login-field-shell">
                  <div className="login-field-core">
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
                      <path d="M7 11V7a5 5 0 0 1 10 0v4" />
                    </svg>
                    <input
                      type="password"
                      value={password}
                      onChange={e => setPassword(e.target.value)}
                      placeholder="请输入密码"
                      autoComplete="current-password"
                    />
                  </div>
                </div>
              </div>

              <div className="login-options">
                <label className="login-remember">
                  <input
                    type="checkbox"
                    checked={remember}
                    onChange={e => setRemember(e.target.checked)}
                  />
                  记住我
                </label>
                <a className="login-forgot" href="#">忘记密码?</a>
              </div>

              {/* Button-in-Button CTA */}
              <button
                type="submit"
                className="login-btn-v2"
                disabled={loading}
              >
                {loading ? '正在登录...' : '登 录'}
                {!loading && (
                  <span className="login-btn-arrow">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                      <line x1="5" y1="12" x2="19" y2="12" />
                      <polyline points="12 5 19 12 12 19" />
                    </svg>
                  </span>
                )}
              </button>
            </form>

            {/* SSO divider + buttons — hidden per requirement */}
            <div className="login-sso-section" style={{ display: 'none' }}>
              <div className="login-divider">or continue with</div>
              <div className="login-sso-row">
                <button className="login-sso-btn" type="button">
                  <svg width="16" height="16" viewBox="0 0 24 24"><path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"/><path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/><path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/><path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/></svg>
                  Google
                </button>
                <button className="login-sso-btn" type="button">
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
                  GitHub
                </button>
                <button className="login-sso-btn" type="button">
                  SSO
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default Login
