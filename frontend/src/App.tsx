import { Routes, Route, Navigate } from 'react-router-dom'
import { lazy, Suspense, useEffect } from 'react'
import { Spin } from 'antd'
import { useAppSelector, useAppDispatch } from './store/hooks'
import { checkAuth } from './store/slices/authSlice'
import Layout from './components/Layout'
import ErrorBoundary from './components/ErrorBoundary'

// 懒加载页面组件
const Login = lazy(() => import('./pages/Login'))
const Dashboard = lazy(() => import('./pages/Dashboard'))
const ClusterList = lazy(() => import('./pages/ClusterList'))
const TopicList = lazy(() => import('./pages/TopicList'))
const ACLList = lazy(() => import('./pages/ACLList'))
const Monitor = lazy(() => import('./pages/Monitor'))
const AuditLog = lazy(() => import('./pages/AuditLog'))
const UserManagement = lazy(() => import('./pages/UserManagement'))
const HostMapping = lazy(() => import('./pages/HostMapping'))

// 懒加载 fallback
const LazyFallback = () => (
  <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%', minHeight: 200 }}>
    <Spin size="large" />
  </div>
)

// 路由权限守卫
const RequireRole: React.FC<{ allowedRoles: string[]; children: React.ReactNode }> = ({ allowedRoles, children }) => {
  const { user } = useAppSelector((state) => state.auth)
  if (!user || !allowedRoles.includes(user.role)) {
    return <Navigate to="/clusters" replace />
  }
  return <>{children}</>
}

function App() {
  const { isAuthenticated, initialized } = useAppSelector((state) => state.auth)
  const dispatch = useAppDispatch()

  useEffect(() => {
    if (!initialized) {
      dispatch(checkAuth())
    }
  }, [dispatch, initialized])

  return (
    <ErrorBoundary>
      <Suspense fallback={<LazyFallback />}>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/*"
            element={
              !initialized ? (
                <LazyFallback />
              ) : isAuthenticated ? (
                <Layout>
                  <Routes>
                    <Route path="/" element={<Navigate to="/dashboard" replace />} />
                    <Route path="/dashboard" element={<Dashboard />} />
                    <Route path="/topics" element={<TopicList />} />
                    <Route path="/monitor" element={<Monitor />} />
                    <Route path="/clusters" element={
                      <RequireRole allowedRoles={['super_admin', 'cluster_admin']}>
                        <ClusterList />
                      </RequireRole>
                    } />
                    <Route path="/acls" element={
                      <RequireRole allowedRoles={['super_admin']}>
                        <ACLList />
                      </RequireRole>
                    } />
                    <Route path="/audit-logs" element={
                      <RequireRole allowedRoles={['super_admin', 'cluster_admin']}>
                        <AuditLog />
                      </RequireRole>
                    } />
                    <Route path="/users" element={
                      <RequireRole allowedRoles={['super_admin']}>
                        <UserManagement />
                      </RequireRole>
                    } />
                    <Route path="/host-mappings" element={
                      <RequireRole allowedRoles={['super_admin']}>
                        <HostMapping />
                      </RequireRole>
                    } />
                  </Routes>
                </Layout>
              ) : (
                <Navigate to="/login" replace />
              )
            }
          />
        </Routes>
      </Suspense>
    </ErrorBoundary>
  )
}

export default App