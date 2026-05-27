import { useState, useEffect } from 'react'
import { Modal, Select, Table, Button, Space, message, Popconfirm } from 'antd'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import { topicPermissionService, TopicPermission } from '../services/topicPermission'
import { clusterAPI } from '../services/cluster'
import { topicService } from '../services/topic'

interface Cluster {
  cluster_id: number
  cluster_name: string
}

interface User {
  user_id: number
  username: string
}

interface TopicPermissionModalProps {
  visible: boolean
  user: User | null
  onClose: () => void
  onSuccess: () => void
}

const TopicPermissionModal: React.FC<TopicPermissionModalProps> = ({
  visible,
  user,
  onClose,
  onSuccess,
}) => {
  const [loading, setLoading] = useState(false)
  const [clusters, setClusters] = useState<Cluster[]>([])
  const [topics, setTopics] = useState<string[]>([])
  const [existingPermissions, setExistingPermissions] = useState<TopicPermission[]>([])
  const [selectedCluster, setSelectedCluster] = useState<number | null>(null)
  const [selectedTopics, setSelectedTopics] = useState<string[]>([])
  const [topicsLoading, setTopicsLoading] = useState(false)

  useEffect(() => {
    if (visible) {
      loadClusters()
    }
  }, [visible])

  useEffect(() => {
    if (visible && user) {
      loadExistingPermissions()
    }
  }, [visible, user])

  useEffect(() => {
    if (selectedCluster) {
      loadTopics(selectedCluster)
    }
  }, [selectedCluster])

  const loadClusters = async () => {
    try {
      const res = await clusterAPI.list()
      setClusters(res.data || [])
    } catch (error) {
      console.error('Failed to load clusters', error)
    }
  }

  const loadTopics = async (clusterId: number) => {
    setTopicsLoading(true)
    try {
      const res = await topicService.list(1, 1000, clusterId)
      const topicNames = (res.data || []).map((t: any) => t.topic_name)
      setTopics(topicNames)
    } catch (error) {
      console.error('Failed to load topics', error)
    } finally {
      setTopicsLoading(false)
    }
  }

  const loadExistingPermissions = async () => {
    if (!user) return
    try {
      const res = await topicPermissionService.getUserPermissions(user.user_id)
      setExistingPermissions(res.data || [])
    } catch (error) {
      console.error('Failed to load permissions', error)
    }
  }

  const handleAssign = async () => {
    if (!user || !selectedCluster || selectedTopics.length === 0) {
      message.warning('请选择集群和 Topic')
      return
    }
    setLoading(true)
    try {
      await topicPermissionService.batchAssign({
        user_id: user.user_id,
        cluster_id: selectedCluster,
        topic_names: selectedTopics,
      })
      message.success('权限分配成功')
      setSelectedTopics([])
      loadExistingPermissions()
      onSuccess()
    } catch (error: any) {
      message.error(error.response?.data?.error || '分配失败')
    } finally {
      setLoading(false)
    }
  }

  const handleRevoke = async (permission: TopicPermission) => {
    if (!user) return
    try {
      await topicPermissionService.revoke({
        user_id: user.user_id,
        cluster_id: permission.cluster_id,
        topic_name: permission.topic_name,
      })
      message.success('权限撤销成功')
      loadExistingPermissions()
      onSuccess()
    } catch (error: any) {
      message.error(error.response?.data?.error || '撤销失败')
    }
  }

  const columns = [
    {
      title: '集群',
      dataIndex: 'cluster_name',
      key: 'cluster_name',
    },
    {
      title: 'Topic',
      dataIndex: 'topic_name',
      key: 'topic_name',
      render: (v: string) => <span className="text-mono" style={{ fontSize: 12 }}>{v}</span>,
    },
    {
      title: '分配时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (text: string) => <span style={{ color: 'var(--text-tertiary)', fontSize: 13 }}>{text}</span>,
    },
    {
      title: '操作',
      key: 'action',
      width: 100,
      render: (_: any, record: TopicPermission) => (
        <Popconfirm title="确定撤销此权限？" onConfirm={() => handleRevoke(record)}>
          <Button type="link" danger icon={<DeleteOutlined />} size="small">
            撤销
          </Button>
        </Popconfirm>
      ),
    },
  ]

  return (
    <Modal
      title={`分配 Topic 权限 - ${user?.username || ''}`}
      open={visible}
      onCancel={onClose}
      width={800}
      footer={null}
    >
      <div style={{ marginBottom: 16, padding: 16, background: 'var(--content-bg)', borderRadius: 'var(--radius-md)', border: '1px solid var(--content-card-border)' }}>
        <Space wrap>
          <Select
            placeholder="选择集群"
            style={{ width: 200 }}
            onChange={setSelectedCluster}
            options={clusters.map(c => ({ label: c.cluster_name, value: c.cluster_id }))}
          />
          <Select
            mode="multiple"
            placeholder="选择 Topic"
            style={{ width: 300 }}
            loading={topicsLoading}
            value={selectedTopics}
            onChange={setSelectedTopics}
            options={topics.map(t => ({ label: t, value: t }))}
            showSearch
            filterOption={(input, option) =>
              (option?.label as string)?.toLowerCase().includes(input.toLowerCase())
            }
          />
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleAssign}
            loading={loading}
            disabled={!selectedCluster || selectedTopics.length === 0}
          >
            分配
          </Button>
        </Space>
      </div>

      <Table
        columns={columns}
        dataSource={existingPermissions}
        rowKey="id"
        size="small"
        pagination={false}
        locale={{ emptyText: '暂无分配的 Topic 权限' }}
      />
    </Modal>
  )
}

export default TopicPermissionModal
