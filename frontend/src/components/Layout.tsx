import { useState, useEffect, useRef, useCallback } from 'react'
import { Layout as AntLayout, Menu, Avatar, Dropdown, theme, message } from 'antd'
import { useNavigate, useLocation } from 'react-router-dom'
import {
  DashboardOutlined,
  ClusterOutlined,
  FileTextOutlined,
  LockOutlined,
  LineChartOutlined,
  AuditOutlined,
  TeamOutlined,
  LogoutOutlined,
  UserOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
} from '@ant-design/icons'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { logoutAsync } from '../store/slices/authSlice'
import api from '../services/api'

const { Header, Sider, Content } = AntLayout

// 默认15分钟，实际值从后端 API 获取
const DEFAULT_IDLE_TIMEOUT = 15 * 60 * 1000

interface LayoutProps {
  children: React.ReactNode
}

const Layout: React.FC<LayoutProps> = ({ children }) => {
  const [collapsed, setCollapsed] = useState(false)
  const navigate = useNavigate()
  const location = useLocation()
  const dispatch = useAppDispatch()
  const { user } = useAppSelector((state) => state.auth)
  const { token } = theme.useToken()
  const idleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const idleTimeoutRef = useRef<number>(DEFAULT_IDLE_TIMEOUT)

  // 根据用户角色决定显示哪些菜单
  const userRole = user?.role || ''
  const isSuperAdmin = userRole === 'super_admin'
  const isClusterAdmin = userRole === 'cluster_admin'

  const menuItems = [
    { key: '/dashboard', icon: <DashboardOutlined />, label: '仪表盘' },
    // 集群管理：超级管理员和集群管理员可见
    ...((isSuperAdmin || isClusterAdmin) ? [{ key: '/clusters', icon: <ClusterOutlined />, label: '集群管理' }] : []),
    // Topic 管理：所有角色可见
    { key: '/topics', icon: <FileTextOutlined />, label: 'Topic 管理' },
    // ACL 管理：仅超级管理员可见
    ...(isSuperAdmin ? [{ key: '/acls', icon: <LockOutlined />, label: 'ACL 管理' }] : []),
    // 监控中心：所有角色可见
    { key: '/monitor', icon: <LineChartOutlined />, label: '监控中心' },
    // 审计日志：超级管理员和集群管理员可见
    ...((isSuperAdmin || isClusterAdmin) ? [{ key: '/audit-logs', icon: <AuditOutlined />, label: '审计日志' }] : []),
    // 用户管理：仅超级管理员可见
    ...(isSuperAdmin ? [{ key: '/users', icon: <TeamOutlined />, label: '用户管理' }] : []),
  ]

  const handleMenuClick = (key: string) => {
    navigate(key)
  }

  // 从后端获取系统配置
  useEffect(() => {
    const fetchConfig = async () => {
      try {
        const res = await api.get('/system/config')
        if (res.data?.idle_timeout) {
          idleTimeoutRef.current = res.data.idle_timeout * 60 * 1000 // 分钟转毫秒
          // 配置更新后重新启动定时器
          if (idleTimerRef.current) {
            clearTimeout(idleTimerRef.current)
          }
          idleTimerRef.current = setTimeout(() => {
            message.warning('登录已过期，请重新登录')
            dispatch(logoutAsync())
            navigate('/login')
          }, idleTimeoutRef.current)
        }
      } catch (error) {
        console.error('Failed to fetch system config:', error)
      }
    }
    fetchConfig()
  }, [dispatch, navigate])

  // 无操作自动登出
  const resetIdleTimer = useCallback(() => {
    if (idleTimerRef.current) {
      clearTimeout(idleTimerRef.current)
    }
    idleTimerRef.current = setTimeout(() => {
      message.warning('登录已过期，请重新登录')
      dispatch(logoutAsync())
      navigate('/login')
    }, idleTimeoutRef.current)
  }, [dispatch, navigate])

  useEffect(() => {
    // 优化：仅监听低频事件，避免 mousemove/scroll 的性能开销
    // visibilitychange 负责检测 Tab 切换，click/keydown 负责检测用户活跃
    const handleActivity = () => resetIdleTimer()
    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
        // Tab 恢复可见时重置计时器（避免切 Tab 期间超时被踢）
        resetIdleTimer()
      }
    }

    window.addEventListener('click', handleActivity)
    window.addEventListener('keydown', handleActivity)
    document.addEventListener('visibilitychange', handleVisibility)
    resetIdleTimer()

    return () => {
      window.removeEventListener('click', handleActivity)
      window.removeEventListener('keydown', handleActivity)
      document.removeEventListener('visibilitychange', handleVisibility)
      if (idleTimerRef.current) {
        clearTimeout(idleTimerRef.current)
      }
    }
  }, [resetIdleTimer])

  const handleLogout = () => {
    dispatch(logoutAsync())
    navigate('/login')
  }

  const userMenuItems = [
    {
      key: 'profile',
      icon: <UserOutlined />,
      label: user?.username || '用户',
    },
    {
      key: 'divider',
      type: 'divider' as const,
    },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      onClick: handleLogout,
    },
  ]

  return (
    <AntLayout className="app-layout" style={{ minHeight: '100vh' }}>
      <Sider
        trigger={null}
        collapsible
        collapsed={collapsed}
        style={{ background: '#001529' }}
      >
        <div
          style={{
            height: 64,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#fff',
            fontSize: collapsed ? 14 : 18,
            fontWeight: 'bold',
          }}
        >
          {collapsed ? 'KMP' : 'Kafka 管理平台'}
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[location.pathname]}
          items={menuItems}
          onClick={({ key }) => handleMenuClick(key)}
          style={{ background: '#001529' }}
        />
      </Sider>
      <AntLayout>
        <Header
          style={{
            padding: '0 24px',
            background: '#fff',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <div
            onClick={() => setCollapsed(!collapsed)}
            style={{ fontSize: 18, cursor: 'pointer' }}
          >
            {collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
          </div>
          <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
            <div style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 8 }}>
              <Avatar icon={<UserOutlined />} style={{ backgroundColor: token.colorPrimary }} />
              <span>{user?.username}</span>
            </div>
          </Dropdown>
        </Header>
        <Content
          style={{
            margin: 24,
            padding: 24,
            background: '#fff',
            borderRadius: 8,
            minHeight: 280,
          }}
        >
          {children}
        </Content>
      </AntLayout>
    </AntLayout>
  )
}

export default Layout