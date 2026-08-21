import { useEffect, useState } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { Select, Button, Modal, Form, Input, InputNumber, message, Spin } from 'antd'
import { PlusOutlined, DeleteOutlined, SyncOutlined, SettingOutlined, TeamOutlined } from '@ant-design/icons'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { fetchTopics, createTopic, deleteTopic } from '../store/slices/topicSlice'
import { clusterAPI } from '../services/cluster'
import { topicService } from '../services/topic'
import { StatCard, LabelTag, SearchBar } from '../components/bento'

interface Cluster {
  cluster_id: number
  cluster_name: string
}

const ClusterTopics: React.FC = () => {
  const dispatch = useAppDispatch()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { topics, total, totalPartitions, totalReplicas, loading } = useAppSelector((state) => state.topics)
  const { user } = useAppSelector((state) => state.auth)
  const isNormalUser = user?.role === 'normal_user'
  const [isModalVisible, setIsModalVisible] = useState(false)
  const [form] = Form.useForm()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [clusters, setClusters] = useState<Cluster[]>([])
  const [clustersLoading, setClustersLoading] = useState(false)
  const [selectedClusterId, setSelectedClusterId] = useState<number | null>(() => {
    const cid = searchParams.get('clusterId')
    return cid ? parseInt(cid) : null
  })
  const [syncing, setSyncing] = useState(false)
  const [configModalVisible, setConfigModalVisible] = useState(false)
  const [cgModalVisible, setCgModalVisible] = useState(false)
  const [configTopicName, setConfigTopicName] = useState('')
  const [configData, setConfigData] = useState<any[]>([])
  const [configLoading, setConfigLoading] = useState(false)
  const [cgData, setCgData] = useState<any[]>([])
  const [cgLoading, setCgLoading] = useState(false)
  const [searchText, setSearchText] = useState('')
  const [descModalVisible, setDescModalVisible] = useState(false)
  const [descTopicName, setDescTopicName] = useState('')
  const [descForm] = Form.useForm()

  // 同步 selectedClusterId 到 URL 参数
  useEffect(() => {
    if (selectedClusterId) {
      const params = new URLSearchParams(searchParams)
      params.set('clusterId', String(selectedClusterId))
      setSearchParams(params, { replace: true })
    }
  }, [selectedClusterId])

  useEffect(() => {
    const loadClusters = async () => {
      setClustersLoading(true)
      try {
        const res = await clusterAPI.list(1, 100)
        const clusterList = res.data || []
        setClusters(clusterList)
        // 如果 URL 中有 clusterId 则使用它，否则选择第一个
        const cid = searchParams.get('clusterId')
        if (cid && clusterList.find((c: Cluster) => c.cluster_id === parseInt(cid))) {
          setSelectedClusterId(parseInt(cid))
        } else if (clusterList.length > 0 && !selectedClusterId) {
          setSelectedClusterId(clusterList[0].cluster_id)
        }
      } catch (err) {
        console.error('Failed to load clusters:', err)
      } finally {
        setClustersLoading(false)
      }
    }
    loadClusters()
  }, [])

  useEffect(() => {
    if (selectedClusterId) {
      dispatch(fetchTopics({ page, pageSize, clusterId: selectedClusterId, search: searchText || undefined }))
    }
  }, [dispatch, page, pageSize, selectedClusterId, searchText])

  const handleOpenModal = () => {
    if (!selectedClusterId) {
      message.warning('请先选择集群')
      return
    }
    setIsModalVisible(true)
  }

  useEffect(() => {
    if (isModalVisible && selectedClusterId) {
      form.setFieldsValue({
        cluster_id: selectedClusterId,
        partitions: 1,
        replication_factor: 1,
      })
    }
  }, [isModalVisible, selectedClusterId, form])

  const handleCreate = async () => {
    try {
      const values = await form.validateFields()
      const clusterId = Number(values.cluster_id)
      if (isNaN(clusterId) || clusterId <= 0) {
        message.error('请选择有效的集群')
        return
      }
      const payload = {
        cluster_id: clusterId,
        topic_name: values.topic_name,
        description: values.description || '',
        partitions: Number(values.partitions),
        replication_factor: Number(values.replication_factor),
      }
      await dispatch(createTopic(payload)).unwrap()
      message.success('创建成功')
      setIsModalVisible(false)
      form.resetFields()
      if (selectedClusterId) {
        dispatch(fetchTopics({ page, pageSize, clusterId: selectedClusterId }))
      }
    } catch (error: any) {
      const errorMsg = typeof error === 'string'
        ? error
        : (error?.message || error?.error || '创建失败')
      message.error(errorMsg)
    }
  }

  const handleDelete = async (topicName: string) => {
    if (!selectedClusterId) {
      message.warning('请先选择集群')
      return
    }
    Modal.confirm({
      title: '确认删除',
      content: `确定要删除 Topic "${topicName}" 吗？此操作不可恢复。`,
      onOk: async () => {
        try {
          await dispatch(deleteTopic({ topicName, clusterId: selectedClusterId })).unwrap()
          message.success('删除成功')
        } catch (error: any) {
          const errorMsg = typeof error === 'string'
            ? error
            : (error?.message || error?.error || '删除失败')
          message.error(errorMsg)
        }
      },
    })
  }

  const handleSync = async () => {
    if (!selectedClusterId) {
      message.warning('请先选择集群')
      return
    }
    setSyncing(true)
    try {
      await topicService.sync(selectedClusterId)
      message.success('同步成功')
      dispatch(fetchTopics({ page, pageSize, clusterId: selectedClusterId }))
    } catch (error: any) {
      const errorMsg = error?.response?.data?.error || error?.message || '同步失败'
      message.error(errorMsg)
    } finally {
      setSyncing(false)
    }
  }

  const handleGoToMonitor = (topicName: string) => {
    if (!selectedClusterId) return
    navigate(`/clusters/monitor?clusterId=${selectedClusterId}&tab=topic&topicName=${encodeURIComponent(topicName)}`)
  }

  const handleGoToMonitorWithConsumer = (topicName: string, consumerGroup: string) => {
    if (!selectedClusterId) return
    navigate(`/clusters/monitor?clusterId=${selectedClusterId}&tab=topic&topicName=${encodeURIComponent(topicName)}&consumerGroup=${encodeURIComponent(consumerGroup)}`)
  }

  const handleViewConfig = async (topicName: string) => {
    if (!selectedClusterId) return
    setConfigTopicName(topicName)
    setConfigModalVisible(true)
    setConfigLoading(true)
    setConfigData([])
    try {
      const res = await topicService.getConfig(topicName, selectedClusterId)
      setConfigData(res.data || [])
    } catch (error: any) {
      message.error(error?.response?.data?.error || '获取配置失败')
    } finally {
      setConfigLoading(false)
    }
  }

  const handleViewConsumerGroups = async (topicName: string) => {
    if (!selectedClusterId) return
    setConfigTopicName(topicName)
    setCgModalVisible(true)
    setCgLoading(true)
    setCgData([])
    try {
      const res = await topicService.getConsumerGroups(topicName, selectedClusterId)
      setCgData(res.data || [])
    } catch (error: any) {
      message.error(error?.response?.data?.error || '获取消费组失败')
    } finally {
      setCgLoading(false)
    }
  }

  const handleEditDescription = (topicName: string, currentDesc: string) => {
    setDescTopicName(topicName)
    descForm.setFieldsValue({ description: currentDesc || '' })
    setDescModalVisible(true)
  }

  const handleSaveDescription = async () => {
    try {
      const values = await descForm.validateFields()
      await topicService.updateDescription(descTopicName, selectedClusterId!, values.description)
      message.success('描述更新成功')
      setDescModalVisible(false)
      dispatch(fetchTopics({ page, pageSize, clusterId: selectedClusterId! }))
    } catch (error: any) {
      const errorMsg = error?.response?.data?.error || error?.message || '更新失败'
      message.error(errorMsg)
    }
  }

  const [gridCols] = useState('2fr 1.2fr 0.8fr 0.8fr 0.8fr 260px')

  return (
    <div>
      <div className="page-header">
        <div className="page-header-row">
          <div>
            <h1>Topic 管理</h1>
            <div className="page-accent-line" />
          </div>
          <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
            <Select
              showSearch
              placeholder="选择集群"
              value={selectedClusterId}
              onChange={(value) => { setSelectedClusterId(value); setPage(1) }}
              style={{ width: 200 }}
              loading={clustersLoading}
              optionFilterProp="children"
              filterOption={(input, option) =>
                (option?.children ?? '').toString().toLowerCase().includes(input.toLowerCase())
              }
            >
              {clusters.map(c => (
                <Select.Option key={c.cluster_id} value={c.cluster_id}>{c.cluster_name}</Select.Option>
              ))}
            </Select>
            {!isNormalUser && (
              <>
                <Button icon={<SyncOutlined spin={syncing} />} onClick={handleSync} loading={syncing} disabled={!selectedClusterId}>同步</Button>
                <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenModal} disabled={!selectedClusterId}>创建 Topic</Button>
              </>
            )}
          </div>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 16, marginBottom: 20 }}>
        <StatCard label="TOPIC TOTAL" value={total} />
        <StatCard label="PARTITIONS" value={totalPartitions} />
        <StatCard label="REPLICAS" value={totalReplicas} />
      </div>

      <div style={{ display: 'flex', gap: 12, marginBottom: 20 }}>
        <SearchBar value={searchText} onChange={(val) => { setSearchText(val); setPage(1) }} placeholder="搜索 Topic 名称..." />
        <Select
          value={pageSize}
          onChange={(val) => { setPageSize(val); setPage(1) }}
          style={{ width: 100 }}
          options={[
            { value: 20, label: '20 条/页' },
            { value: 50, label: '50 条/页' },
            { value: 100, label: '100 条/页' },
          ]}
        />
      </div>

      <div className="bento-table-header" style={{ gridTemplateColumns: gridCols }}>
        <div>Topic Name</div>
        <div>Description</div>
        <div style={{ textAlign: 'center' }}>Partitions</div>
        <div style={{ textAlign: 'center' }}>Replicas</div>
        <div style={{ textAlign: 'center' }}>Created</div>
        <div style={{ textAlign: 'right' }}>Actions</div>
      </div>

      <div className="bento-table-body">
        {!selectedClusterId && (
          <div style={{ textAlign: 'center', padding: 48, color: 'var(--text-3)' }}>请先选择集群</div>
        )}
        {selectedClusterId && loading && (
          <div style={{ textAlign: 'center', padding: 48 }}><Spin /></div>
        )}
        {selectedClusterId && !loading && topics.map((topic: any) => (
          <div key={topic.id || topic.topic_name} className="bento-table-row" style={{ gridTemplateColumns: gridCols, cursor: 'pointer' }} onClick={() => handleGoToMonitor(topic.topic_name)}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ color: 'var(--brand)', fontSize: 14, fontWeight: 700 }}>&#9830;</span>
              <a onClick={(e) => { e.stopPropagation(); handleGoToMonitor(topic.topic_name) }} style={{ fontWeight: 700, fontSize: 14, color: 'var(--text-1)', fontFamily: 'var(--font-mono)' }}>{topic.topic_name}</a>
              {topic.topic_name.startsWith('__') && <LabelTag text="SYSTEM" color="warning" />}
            </div>
            <div style={{ fontSize: 12, color: 'var(--text-3)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{topic.description || '-'}</div>
            <div style={{ textAlign: 'center' }}><LabelTag text={String(topic.partitions)} color={topic.topic_name.startsWith('__') ? 'warning' : 'orange'} /></div>
            <div style={{ textAlign: 'center' }}><LabelTag text={String(topic.replication_factor)} color={topic.topic_name.startsWith('__') ? 'warning' : 'green'} /></div>
            <div style={{ textAlign: 'center', fontSize: 12, color: 'var(--text-3)' }}>{topic.created_at}</div>
            <div style={{ textAlign: 'right', display: 'flex', gap: 6, justifyContent: 'flex-end' }}>
              <button className="bento-action-btn" onClick={(e) => { e.stopPropagation(); handleViewConfig(topic.topic_name) }}><SettingOutlined /> 配置</button>
              <button className="bento-action-btn" onClick={(e) => { e.stopPropagation(); handleViewConsumerGroups(topic.topic_name) }}><TeamOutlined /> 消费组</button>
              {!isNormalUser && !topic.topic_name.startsWith('__') && (
                <button className="bento-action-btn" onClick={(e) => { e.stopPropagation(); handleEditDescription(topic.topic_name, topic.description) }}><SettingOutlined /> 描述</button>
              )}
              {!isNormalUser && !topic.topic_name.startsWith('__') && (
                <button className="bento-action-btn bento-action-btn--danger" onClick={(e) => { e.stopPropagation(); handleDelete(topic.topic_name) }}><DeleteOutlined /> 删除</button>
              )}
            </div>
          </div>
        ))}
        {selectedClusterId && !loading && topics.length === 0 && (
          <div style={{ textAlign: 'center', padding: 48, color: 'var(--text-3)' }}>暂无 Topic 数据</div>
        )}
      </div>

      {/* Pagination */}
      {total > pageSize && (
        <div className="bento-pagination">
          <span className="bento-pagination-info">Showing {(page - 1) * pageSize + 1}-{Math.min(page * pageSize, total)} of {total}</span>
          <div className="bento-pagination-buttons">
            <button className="bento-pagination-btn" disabled={page <= 1} onClick={() => setPage(page - 1)}>&larr;</button>
            {Array.from({ length: Math.ceil(total / pageSize) }, (_, i) => i + 1)
              .filter(p => Math.abs(p - page) <= 2)
              .map(p => (
                <button key={p} className={`bento-pagination-btn${p === page ? ' bento-pagination-btn--active' : ''}`} onClick={() => setPage(p)}>{p}</button>
              ))}
            <button className="bento-pagination-btn" disabled={page >= Math.ceil(total / pageSize)} onClick={() => setPage(page + 1)}>&rarr;</button>
          </div>
        </div>
      )}

      {/* Create Topic Modal */}
      <Modal title="创建 Topic" open={isModalVisible} onOk={handleCreate} onCancel={() => { setIsModalVisible(false); form.resetFields() }} destroyOnClose width={600}>
        <Form form={form} layout="vertical">
          <Form.Item name="cluster_id" label="所属集群" rules={[{ required: true }]}>
            <Select placeholder="请选择集群" disabled>
              {clusters.map(c => (<Select.Option key={c.cluster_id} value={c.cluster_id}>{c.cluster_name}</Select.Option>))}
            </Select>
          </Form.Item>
          <Form.Item name="topic_name" label="Topic 名称" rules={[{ required: true, message: '请输入 Topic 名称' }]}>
            <Input placeholder="请输入 Topic 名称" />
          </Form.Item>
          <Form.Item name="description" label="描述"><Input placeholder="请输入描述（可选，用于标识 Topic 用途）" /></Form.Item>
          <Form.Item name="partitions" label="分区数" rules={[{ required: true, message: '请输入分区数' }]} initialValue={1}>
            <InputNumber min={1} max={100} style={{ width: '100%' }} placeholder="请输入分区数" />
          </Form.Item>
          <Form.Item name="replication_factor" label="副本数" rules={[{ required: true, message: '请输入副本数' }]} initialValue={1}>
            <InputNumber min={1} max={10} style={{ width: '100%' }} placeholder="请输入副本数" />
          </Form.Item>
        </Form>
      </Modal>

      {/* Config Modal */}
      <Modal title={`Topic 配置 - ${configTopicName}`} open={configModalVisible} footer={null} onCancel={() => setConfigModalVisible(false)} width={750}>
        <Spin spinning={configLoading}>
          <div className="bento-table-header" style={{ gridTemplateColumns: 'minmax(150px, 1.5fr) minmax(150px, 1.5fr) 100px 60px 60px' }}>
            <div>配置项</div><div>值</div><div>来源</div><div style={{ textAlign: 'center' }}>只读</div><div style={{ textAlign: 'center' }}>默认</div>
          </div>
          <div className="bento-table-body">
            {configData.map((item: any) => (
              <div key={item.name} className="bento-table-row" style={{ gridTemplateColumns: 'minmax(150px, 1.5fr) minmax(150px, 1.5fr) 100px 60px 60px' }}>
                <span className="text-mono bento-table-cell-wrap" style={{ fontSize: 12 }} title={item.name}>{item.name}</span>
                <span className="text-mono bento-table-cell-wrap" style={{ fontSize: 12 }} title={item.value}>{item.value}</span>
                <span style={{ fontSize: 12 }}>{item.source}</span>
                <LabelTag text={item.read_only ? '是' : '否'} color={item.read_only ? 'neutral' : 'green'} />
                <LabelTag text={item.is_default ? '是' : '否'} color={item.is_default ? 'neutral' : 'blue'} />
              </div>
            ))}
          </div>
        </Spin>
      </Modal>

      {/* Consumer Groups Modal */}
      <Modal title={`消费组 - ${configTopicName}`} open={cgModalVisible} footer={null} onCancel={() => setCgModalVisible(false)} width={700}>
        <Spin spinning={cgLoading}>
          {cgData.length === 0 && !cgLoading && (
            <div style={{ textAlign: 'center', color: 'var(--text-3)', padding: 40, fontSize: 13 }}>该 Topic 暂无消费组</div>
          )}
          {cgData.length > 0 && (
            <>
              <div className="bento-table-header" style={{ gridTemplateColumns: 'minmax(200px, 2fr) 100px 80px 100px' }}>
                <div>消费组</div>
                <div>状态</div>
                <div style={{ textAlign: 'center' }}>成员数</div>
                <div style={{ textAlign: 'right' }}>Lag</div>
              </div>
              <div className="bento-table-body">
                {cgData.map((item: any) => (
                  <div key={item.group_id} className="bento-table-row" style={{ gridTemplateColumns: 'minmax(200px, 2fr) 100px 80px 100px' }}>
                    <a className="text-mono bento-table-cell-wrap" style={{ fontSize: 12, fontWeight: 600, wordBreak: 'break-all', color: 'var(--brand)', cursor: 'pointer' }}
                      title={item.group_id}
                      onClick={() => { setCgModalVisible(false); handleGoToMonitorWithConsumer(configTopicName, item.group_id) }}>
                      {item.group_id}
                    </a>
                    <LabelTag text={item.state} color={item.state === 'Stable' ? 'green' : item.state === 'Empty' ? 'orange' : 'red'} />
                    <span style={{ textAlign: 'center', fontSize: 13 }}>{item.member_count}</span>
                    <span style={{ textAlign: 'right', fontSize: 13, fontFamily: 'var(--font-mono)', color: (item.total_lag || 0) > 0 ? '#ef4444' : 'var(--text-3)' }}>
                      {(item.total_lag || 0).toLocaleString()}
                    </span>
                  </div>
                ))}
              </div>
            </>
          )}
        </Spin>
      </Modal>

      {/* Edit Description Modal */}
      <Modal title={`编辑描述 - ${descTopicName}`} open={descModalVisible} onOk={handleSaveDescription} onCancel={() => setDescModalVisible(false)} destroyOnClose>
        <Form form={descForm} layout="vertical">
          <Form.Item name="description" label="描述"><Input.TextArea placeholder="请输入描述，用于标识 Topic 用途（如：用户行为日志、订单数据等）" rows={3} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default ClusterTopics