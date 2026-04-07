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
                <Route path="/clusters" element={<ClusterList />} />
                <Route path="/topics" element={<TopicList />} />
                <Route path="/acls" element={<ACLList />} />
                <Route path="/monitor" element={<Monitor />} />
                <Route path="/audit-logs" element={<AuditLog />} />
                <Route path="/users" element={<UserManagement />} />
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