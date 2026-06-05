import { useState, useEffect, useRef, useCallback } from 'react'
import { Layout as AntLayout, Menu, Avatar, Dropdown, message } from 'antd'
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

// 路由到面包屑映射
const routeBreadcrumb: Record<string, string> = {
  '/dashboard': '仪表盘',
  '/clusters': '集群管理',
  '/topics': 'Topic 管理',
  '/acls': 'ACL 管理',
  '/monitor': '监控中心',
  '/audit-logs': '审计日志',
  '/users': '用户管理',
}

interface LayoutProps {
  children: React.ReactNode
}

const Layout: React.FC<LayoutProps> = ({ children }) => {
  const [collapsed, setCollapsed] = useState(false)
  const navigate = useNavigate()
  const location = useLocation()
  const dispatch = useAppDispatch()
  const { user } = useAppSelector((state) => state.auth)
  const idleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const idleTimeoutRef = useRef<number>(DEFAULT_IDLE_TIMEOUT)

  // 根据用户角色决定显示哪些菜单
  const userRole = user?.role || ''
  const isSuperAdmin = userRole === 'super_admin'
  const isClusterAdmin = userRole === 'cluster_admin'

  const menuItems = [
    { key: '/dashboard', icon: <DashboardOutlined />, label: '仪表盘' },
    ...((isSuperAdmin || isClusterAdmin) ? [{ key: '/clusters', icon: <ClusterOutlined />, label: '集群管理' }] : []),
    { key: '/topics', icon: <FileTextOutlined />, label: 'Topic 管理' },
    ...(isSuperAdmin ? [{ key: '/acls', icon: <LockOutlined />, label: 'ACL 管理' }] : []),
    { key: '/monitor', icon: <LineChartOutlined />, label: '监控中心' },
    ...((isSuperAdmin || isClusterAdmin) ? [{ key: '/audit-logs', icon: <AuditOutlined />, label: '审计日志' }] : []),
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
          idleTimeoutRef.current = res.data.idle_timeout * 60 * 1000
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
    const handleActivity = () => resetIdleTimer()
    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
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

  const currentPageName = routeBreadcrumb[location.pathname] || ''

  return (
    <AntLayout className="app-layout" style={{ minHeight: '100dvh' }}>
      <Sider
        trigger={null}
        collapsible
        collapsed={collapsed}
        className="app-sidebar"
        width={240}
        collapsedWidth={72}
      >
        {/* 品牌区 */}
        <div className="sidebar-brand">
          <div className="sidebar-brand-icon">Km</div>
          {!collapsed && (
            <div className="sidebar-brand-text">
              <span>K</span>manager
            </div>
          )}
        </div>
        {/* 菜单 */}
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[location.pathname]}
          items={menuItems}
          onClick={({ key }) => handleMenuClick(key)}
        />
      </Sider>
      <AntLayout>
        <Header className="app-header">
          <div className="header-left">
            <div className="header-trigger" onClick={() => setCollapsed(!collapsed)}>
              {collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            </div>
            {currentPageName && (
              <div className="header-breadcrumb">
                <strong>{currentPageName}</strong>
              </div>
            )}
          </div>
          <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
            <div className="header-user">
              <Avatar icon={<UserOutlined />} className="header-user-avatar" size={30} />
              <span className="header-user-name">{user?.username}</span>
            </div>
          </Dropdown>
        </Header>
        <Content className="app-content">
          {children}
        </Content>
      </AntLayout>
    </AntLayout>
  )
}

export default Layout
