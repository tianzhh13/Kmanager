import { useState, useEffect, useRef, useCallback } from 'react'
import { Layout as AntLayout, Menu, Avatar, Dropdown, Modal, Form, Input, message } from 'antd'
import { useNavigate, useLocation } from 'react-router-dom'
import {
	  DashboardOutlined,
	  ClusterOutlined,
	  AuditOutlined,
	  TeamOutlined,
	  LogoutOutlined,
	  UserOutlined,
	  MenuFoldOutlined,
	  MenuUnfoldOutlined,
	  SwapOutlined,
	  KeyOutlined,
	} from '@ant-design/icons'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { logoutAsync } from '../store/slices/authSlice'
import api from '../services/api'
import { userService } from '../services/userService'

const { Header, Sider, Content } = AntLayout

// 默认15分钟，实际值从后端 API 获取
const DEFAULT_IDLE_TIMEOUT = 15 * 60 * 1000

// 路由到面包屑映射
const routeBreadcrumb: Record<string, string> = {
  '/dashboard': '仪表盘',
  '/clusters': '集群管理',
  '/clusters/topics': '集群管理 - Topic 管理',
  '/clusters/acls': '集群管理 - ACL 管理',
  '/clusters/monitor': '集群管理 - 监控中心',
  '/topics': 'Topic 管理',
  '/acls': 'ACL 管理',
  '/monitor': '监控中心',
  '/audit-logs': '审计日志',
  '/users': '用户管理',
  '/host-mappings': '主机映射',
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
  const [pwdModalVisible, setPwdModalVisible] = useState(false)
  const [pwdForm] = Form.useForm()
  const [pwdLoading, setPwdLoading] = useState(false)

  // 根据用户角色决定显示哪些菜单
  const userRole = user?.role || ''
  const isSuperAdmin = userRole === 'super_admin'
  const isClusterAdmin = userRole === 'cluster_admin'

  const menuItems = [
    { key: '/dashboard', icon: <DashboardOutlined />, label: '仪表盘' },
    {
      key: '/clusters',
      icon: <ClusterOutlined />,
      label: '集群管理',
      children: [
        { key: '/clusters', label: '集群列表' },
        { key: '/clusters/topics', label: 'Topic 管理' },
        ...((isSuperAdmin || isClusterAdmin) ? [{ key: '/clusters/acls', label: 'ACL 管理' }] : []),
        { key: '/clusters/monitor', label: '监控中心' },
      ],
    },
    ...((isSuperAdmin || isClusterAdmin) ? [{ key: '/audit-logs', icon: <AuditOutlined />, label: '审计日志' }] : []),
    ...(isSuperAdmin ? [{ key: '/users', icon: <TeamOutlined />, label: '用户管理' }] : []),
    ...(isSuperAdmin ? [{ key: '/host-mappings', icon: <SwapOutlined />, label: '主机映射' }] : []),
  ]

  const handleMenuClick = (key: string) => {
    // 从当前 URL 提取 clusterId 参数，传递给新页面保持集群选中状态
    const params = new URLSearchParams(location.search)
    const clusterId = params.get('clusterId')
    if (clusterId && key.startsWith('/clusters/')) {
      navigate(`${key}?clusterId=${clusterId}`)
    } else {
      navigate(key)
    }
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

  const handleOpenPwdModal = () => {
    setPwdModalVisible(true)
    pwdForm.resetFields()
  }

  const handleChangePassword = async (values: { old_password: string; new_password: string; confirm_password: string }) => {
    if (values.new_password !== values.confirm_password) {
      message.error('两次输入的新密码不一致')
      return
    }
    if (!user?.user_id) {
      message.error('无法获取用户信息')
      return
    }
    setPwdLoading(true)
    try {
      await userService.updatePassword(user.user_id, {
        old_password: values.old_password,
        new_password: values.new_password,
      })
      message.success('密码修改成功')
      setPwdModalVisible(false)
      pwdForm.resetFields()
    } catch (error: any) {
      message.error(error?.response?.data?.error || '密码修改失败')
    } finally {
      setPwdLoading(false)
    }
  }

  const userMenuItems = [
    {
      key: 'profile',
      icon: <UserOutlined />,
      label: user?.username || '用户',
      disabled: true,
    },
    {
      key: 'divider1',
      type: 'divider' as const,
    },
    {
      key: 'change-password',
      icon: <KeyOutlined />,
      label: '修改密码',
      onClick: handleOpenPwdModal,
    },
    {
      key: 'divider2',
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
          defaultOpenKeys={location.pathname.startsWith('/clusters') ? ['/clusters'] : []}
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

      {/* 修改密码弹窗 */}
      <Modal
        title="修改密码"
        open={pwdModalVisible}
        onCancel={() => { setPwdModalVisible(false); pwdForm.resetFields() }}
        onOk={() => pwdForm.submit()}
        confirmLoading={pwdLoading}
        width={420}
      >
        <Form form={pwdForm} layout="vertical" onFinish={handleChangePassword}>
          <Form.Item name="old_password" label="当前密码" rules={[{ required: true, message: '请输入当前密码' }]}>
            <Input.Password placeholder="请输入当前密码" />
          </Form.Item>
          <Form.Item name="new_password" label="新密码" rules={[
            { required: true, message: '请输入新密码' },
            { min: 8, message: '密码长度至少 8 个字符' },
          ]}>
            <Input.Password placeholder="至少 8 个字符" />
          </Form.Item>
          <Form.Item name="confirm_password" label="确认新密码" rules={[
            { required: true, message: '请再次输入新密码' },
          ]}>
            <Input.Password placeholder="再次输入新密码" />
          </Form.Item>
        </Form>
      </Modal>
    </AntLayout>
  )
}

export default Layout
