import { Routes, Route, Navigate } from 'react-router-dom'
import { useAppSelector } from './store/hooks'
import Layout from './components/Layout'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import ClusterList from './pages/ClusterList'
import TopicList from './pages/TopicList'
import ACLList from './pages/ACLList'
import Monitor from './pages/Monitor'
import AuditLog from './pages/AuditLog'
import UserManagement from './pages/UserManagement'

// 路由权限守卫
const RequireRole: React.FC<{ allowedRoles: string[]; children: React.ReactNode }> = ({ allowedRoles, children }) => {
  const { user } = useAppSelector((state) => state.auth)
  if (!user || !allowedRoles.includes(user.role)) {
    return <Navigate to="/dashboard" replace />
  }
  return <>{children}</>
}

function App() {
  const { isAuthenticated } = useAppSelector((state) => state.auth)

  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        path="/*"
        element={
          isAuthenticated ? (
            <Layout>
              <Routes>
                <Route path="/" element={<Navigate to="/dashboard" replace />} />
                <Route path="/dashboard" element={<Dashboard />} />
                <Route path="/topics" element={<TopicList />} />
                <Route path="/monitor" element={<Monitor />} />
                {/* 集群管理：超级管理员 + 集群管理员 */}
                <Route path="/clusters" element={
                  <RequireRole allowedRoles={['super_admin', 'cluster_admin']}>
                    <ClusterList />
                  </RequireRole>
                } />
                {/* ACL 管理：仅超级管理员 */}
                <Route path="/acls" element={
                  <RequireRole allowedRoles={['super_admin']}>
                    <ACLList />
                  </RequireRole>
                } />
                {/* 审计日志：超级管理员 + 集群管理员 */}
                <Route path="/audit-logs" element={
                  <RequireRole allowedRoles={['super_admin', 'cluster_admin']}>
                    <AuditLog />
                  </RequireRole>
                } />
                {/* 用户管理：仅超级管理员 */}
                <Route path="/users" element={
                  <RequireRole allowedRoles={['super_admin']}>
                    <UserManagement />
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
  )
}

export default App