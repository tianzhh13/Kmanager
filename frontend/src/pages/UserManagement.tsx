import { useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, Tag, message } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, StopOutlined, CheckCircleOutlined } from '@ant-design/icons'

const UserManagement: React.FC = () => {
  const [loading] = useState(false)
  const [isModalVisible, setIsModalVisible] = useState(false)
  const [form] = Form.useForm()

  const handleCreate = async () => {
    try {
      await form.validateFields()
      message.success('创建成功')
      setIsModalVisible(false)
      form.resetFields()
    } catch (error) {
      // validation failed
    }
  }

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
    { title: '用户名', dataIndex: 'username', key: 'username' },
    { title: '邮箱', dataIndex: 'email', key: 'email' },
    { title: '角色', dataIndex: 'role', key: 'role',
      render: (role: string) => {
        const colorMap: Record<string, string> = {
          super_admin: 'red',
          cluster_admin: 'blue',
          readonly: 'green',
        }
        const labelMap: Record<string, string> = {
          super_admin: '超级管理员',
          cluster_admin: '集群管理员',
          readonly: '只读用户',
        }
        return <Tag color={colorMap[role] || 'default'}>{labelMap[role] || role}</Tag>
      }
    },
    { title: '状态', dataIndex: 'status', key: 'status',
      render: (status: string) => (
        <Tag color={status === 'active' ? 'success' : 'error'}>
          {status === 'active' ? '活跃' : '禁用'}
        </Tag>
      )
    },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at' },
    { title: '操作', key: 'action', width: 200,
      render: () => (
        <Space>
          <Button type="link" icon={<EditOutlined />}>编辑</Button>
          <Button type="link" icon={<StopOutlined />}>禁用</Button>
          <Button type="link" danger icon={<DeleteOutlined />}>删除</Button>
        </Space>
      )
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1>用户管理</h1>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsModalVisible(true)}>
          创建用户
        </Button>
      </div>

      <Table
        columns={columns}
        dataSource={[]}
        rowKey="id"
        loading={loading}
        pagination={{ pageSize: 20 }}
        locale={{ emptyText: '暂无用户数据' }}
      />

      <Modal
        title="创建用户"
        open={isModalVisible}
        onOk={handleCreate}
        onCancel={() => { setIsModalVisible(false); form.resetFields() }}
        width={500}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input placeholder="3-64个字符" />
          </Form.Item>
          <Form.Item name="email" label="邮箱" rules={[{ required: true, type: 'email', message: '请输入有效邮箱' }]}>
            <Input placeholder="user@example.com" />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password placeholder="至少8个字符" />
          </Form.Item>
          <Form.Item name="role" label="角色" rules={[{ required: true }]}>
            <Select placeholder="选择角色">
              <Select.Option value="super_admin">超级管理员</Select.Option>
              <Select.Option value="cluster_admin">集群管理员</Select.Option>
              <Select.Option value="readonly">只读用户</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default UserManagement