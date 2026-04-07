import { useState } from 'react'
import { Table, Card, Select, DatePicker, Space, Input, Tag } from 'antd'
import { SearchOutlined, ExportOutlined } from '@ant-design/icons'

const { RangePicker } = DatePicker

const AuditLog: React.FC = () => {
  const [loading] = useState(false)

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
    { title: '用户', dataIndex: 'username', key: 'username' },
    { title: '操作', dataIndex: 'action', key: 'action' },
    { title: '资源类型', dataIndex: 'resource_type', key: 'resource_type' },
    { title: '资源ID', dataIndex: 'resource_id', key: 'resource_id' },
    { title: '状态', dataIndex: 'status', key: 'status',
      render: (status: string) => (
        <Tag color={status === 'success' ? 'success' : 'error'}>
          {status === 'success' ? '成功' : '失败'}
        </Tag>
      )
    },
    { title: 'IP地址', dataIndex: 'ip_address', key: 'ip_address' },
    { title: '时间', dataIndex: 'created_at', key: 'created_at' },
  ]

  return (
    <div>
      <h1 style={{ marginBottom: 24 }}>审计日志</h1>
      
      <Card style={{ marginBottom: 16 }}>
        <Space>
          <Input placeholder="搜索用户名" prefix={<SearchOutlined />} style={{ width: 200 }} />
          <Select placeholder="操作类型" style={{ width: 150 }} allowClear>
            <Select.Option value="login">登录</Select.Option>
            <Select.Option value="create">创建</Select.Option>
            <Select.Option value="update">更新</Select.Option>
            <Select.Option value="delete">删除</Select.Option>
          </Select>
          <Select placeholder="资源类型" style={{ width: 150 }} allowClear>
            <Select.Option value="cluster">集群</Select.Option>
            <Select.Option value="topic">Topic</Select.Option>
            <Select.Option value="acl">ACL</Select.Option>
          </Select>
          <RangePicker showTime />
          <Button icon={<ExportOutlined />}>导出</Button>
        </Space>
      </Card>

      <Card>
        <Table
          columns={columns}
          dataSource={[]}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 20 }}
          locale={{ emptyText: '暂无审计日志' }}
        />
      </Card>
    </div>
  )
}

export default AuditLog