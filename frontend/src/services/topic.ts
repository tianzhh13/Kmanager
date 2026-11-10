import api from './api'

export interface Topic {
  id: number
  cluster_id: number
  topic_name: string
  partitions: number
  replication_factor: number
  config: Record<string, string>
  created_at: string
  updated_at: string
}

export interface CreateTopicRequest {
  cluster_id: number
  topic_name: string
  description?: string
  partitions: number
  replication_factor: number
  config?: Record<string, string>
}

export interface UpdateTopicConfigRequest {
  config: Record<string, string>
}

export const topicService = {
  list: async (page: number = 1, pageSize: number = 20, clusterId?: number, search?: string) => {
    const response = await api.get('/topics', {
      params: { page, page_size: pageSize, cluster_id: clusterId, search },
    })
    return response.data
  },

  get: async (topicName: string): Promise<Topic> => {
    const response = await api.get(`/topics/${encodeURIComponent(topicName)}`)
    return response.data
  },

  create: async (data: CreateTopicRequest): Promise<Topic> => {
    const response = await api.post('/topics', data)
    return response.data
  },

  delete: async (topicName: string, clusterId: number): Promise<void> => {
    await api.delete(`/topics/${encodeURIComponent(topicName)}`, {
      params: { cluster_id: clusterId }
    })
  },

  updateConfig: async (topicName: string, data: UpdateTopicConfigRequest): Promise<void> => {
    await api.put(`/topics/${encodeURIComponent(topicName)}/config`, data)
  },

  sync: async (clusterId: number): Promise<void> => {
    const response = await api.post(`/topics/sync/${clusterId}`)
    return response.data
  },

  getConfig: async (topicName: string, clusterId: number) => {
    const response = await api.get(`/topics/${encodeURIComponent(topicName)}/config`, {
      params: { cluster_id: clusterId },
    })
    return response.data
  },

  getConsumerGroups: async (topicName: string, clusterId: number) => {
    const response = await api.get(`/topics/${encodeURIComponent(topicName)}/consumer-groups`, {
      params: { cluster_id: clusterId },
    })
    return response.data
  },

  updateDescription: async (topicName: string, clusterId: number, description: string): Promise<void> => {
    await api.put(`/topics/${encodeURIComponent(topicName)}/description`, { description }, {
      params: { cluster_id: clusterId },
    })
  },

  // 批量校验 topic 名称（存在性 + '.'/'_' 命名冲突），工单批量导入预检使用
  checkNames: async (clusterId: number, names: string[]): Promise<Record<string, TopicNameCheckResult>> => {
    const response = await api.post('/topics/check-names', { cluster_id: clusterId, names })
    return response.data.results || {}
  },
}

// topic 名称校验结果（与后端 TopicNameCheckResult 对应）
export interface TopicNameCheckResult {
  exists: boolean
  conflict_with: string
}