import api from './api'

// 消息记录（与后端 kafka.MessageRecord 对应）
export interface MessageRecord {
  partition: number
  offset: number
  timestamp: number // unix milliseconds
  key: string
  // 后端返回 base64 编码的原始字节
  value: string
  headers: MessageHeader[]
}

export interface MessageHeader {
  key: string
  value: string
}

// 查询模式
export type MessageQueryType = 'offset' | 'timestamp' | 'key' | 'latest' | 'earliest'

// 消息查询请求（与后端 message.QueryMessagesRequest 对应）
export interface QueryMessagesRequest {
  partition: number
  query_type: MessageQueryType
  start_offset?: number
  end_offset?: number
  timestamp?: number // 毫秒
  key?: string
  max_count?: number
}

// 消息查询响应（与后端 message.QueryMessagesResponse 对应）
export interface QueryMessagesResponse {
  data: MessageRecord[]
  partition_offsets: {
    start: number
    end: number
  }
  total: number
}

export const messageService = {
  // 查询 topic 消息（按 offset / 时间戳 / key 等）
  query: async (topicName: string, clusterId: number, req: QueryMessagesRequest): Promise<QueryMessagesResponse> => {
    const response = await api.post(`/topics/${encodeURIComponent(topicName)}/messages`, req, {
      params: { cluster_id: clusterId },
    })
    return response.data
  },
}
