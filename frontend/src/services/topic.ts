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
  partitions: number
  replication_factor: number
  config?: Record<string, string>
}

export interface UpdateTopicConfigRequest {
  config: Record<string, string>
}

export const topicService = {
  list: async (page: number = 1, pageSize: number = 20, clusterId?: number) => {
    const response = await api.get('/topics', {
      params: { page, page_size: pageSize, cluster_id: clusterId },
    })
    return response.data
  },

  get: async (topicName: string): Promise<Topic> => {
    const response = await api.get(`/topics/${encodeURIComponent(topicName)}`)
    return response.data
  },

  create: async (data: CreateTopicRequest): Promise<Topic> => {
    console.log('=== DEBUG: topicService.create called with ===', JSON.stringify(data, null, 2))
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
    console.log('=== DEBUG: topicService.sync called, clusterId ===', clusterId)
    const response = await api.post(`/topics/sync/${clusterId}`)
    console.log('=== DEBUG: sync response ===', response)
    return response.data
  },
}