import api from './api'

// 工单类型
export type TicketType = 'topic_create' | 'permission_grant'

// 工单状态
export type TicketStatus = 'pending' | 'executing' | 'succeeded' | 'failed' | 'rejected' | 'cancelled'

// 工单
export interface Ticket {
  ticket_id: number
  ticket_no: string
  type: TicketType
  cluster_id: number
  applicant_id: number
  title: string
  form: string
  status: TicketStatus
  designated_approver_id: number | null
  designated_approver_name?: string
  applicant_name?: string
  approver_name?: string
  approver_id: number | null
  approve_remark: string
  result: string | null
  credential_viewed_at: string | null
  error_msg: string
  applied_at: string
  approved_at: string | null
  completed_at: string | null
  created_at: string
  updated_at: string
}

// 申请 Topic 表单
export interface TopicCreateForm {
  topic_name: string
  partitions: number
  replication_factor: number
  retention_days: number
  business_desc: string
}

// 申请权限表单
export interface PermissionGrantForm {
  mode: 'new_user' | 'existing_user'
  username: string
  mechanism: 'SCRAM-SHA-256' | 'SCRAM-SHA-512'
  permission: 'read' | 'write' | 'read_write'
  topics: string[]
  topic_pattern: 'literal' | 'prefixed'
  group_prefix: string
  extras: string[]
}

// 工单列表请求
export interface TicketListParams {
  scope: 'mine' | 'pending' | 'all'
  status?: string
  type?: string
  cluster_id?: number
  page: number
  page_size: number
}

// 工单列表响应
export interface TicketListResponse {
  data: Ticket[]
  total: number
}

// ACL 展开预览规则（与后端 ExpandACLs 一致）
export interface ACLPreviewRule {
  resource_type: string
  resource_name: string
  resource_pattern: string
  operation: string
}

// 执行结果摘要
export interface TicketExecuteResult {
  created?: string[]
  skipped?: string[]
}

export const ticketService = {
  list: async (params: TicketListParams): Promise<TicketListResponse> => {
    const response = await api.get('/tickets', { params })
    return response.data
  },

  stats: async (): Promise<Record<string, number>> => {
    const response = await api.get('/tickets/stats')
    return response.data
  },

  get: async (id: number): Promise<Ticket> => {
    const response = await api.get(`/tickets/${id}`)
    return response.data
  },

  submit: async (data: { type: TicketType; cluster_id: number; form: TopicCreateForm | PermissionGrantForm }): Promise<Ticket> => {
    const response = await api.post('/tickets', data)
    return response.data
  },

  approve: async (id: number, remark?: string): Promise<Ticket> => {
    const response = await api.post(`/tickets/${id}/approve`, { remark: remark || '' })
    return response.data
  },

  reject: async (id: number, remark: string): Promise<Ticket> => {
    const response = await api.post(`/tickets/${id}/reject`, { remark })
    return response.data
  },

  cancel: async (id: number): Promise<Ticket> => {
    const response = await api.post(`/tickets/${id}/cancel`)
    return response.data
  },

  retry: async (id: number): Promise<Ticket> => {
    const response = await api.post(`/tickets/${id}/retry`)
    return response.data
  },

  takeCredential: async (id: number): Promise<{ username: string; password: string; mechanism: string }> => {
    const response = await api.post(`/tickets/${id}/credentials`)
    return response.data
  },
}

// 展开权限表单为 ACL 明细（与后端 ExpandACLs 规则一致，用于提交前预览）
export function expandACLPreview(form: PermissionGrantForm): ACLPreviewRule[] {
  const rules: ACLPreviewRule[] = []
  const hasRead = form.permission === 'read' || form.permission === 'read_write'
  const hasWrite = form.permission === 'write' || form.permission === 'read_write'

  const topicOps: string[] = []
  if (hasRead) topicOps.push('read', 'describe')
  if (hasWrite) topicOps.push('write', 'describe')
  if (form.extras?.includes('describeconfigs')) topicOps.push('describeconfigs')

  for (const topic of form.topics) {
    for (const op of topicOps) {
      rules.push({
        resource_type: 'topic',
        resource_name: topic,
        resource_pattern: form.topic_pattern,
        operation: op,
      })
    }
  }

  if (hasRead) {
    const isWildcard = form.group_prefix === '*'
    const groupOps = ['read']
    if (form.extras?.includes('group_describe')) groupOps.push('describe')
    for (const op of groupOps) {
      rules.push({
        resource_type: 'group',
        resource_name: isWildcard ? '*' : form.group_prefix,
        resource_pattern: isWildcard ? 'literal' : 'prefixed',
        operation: op,
      })
    }
  }

  return rules
}
