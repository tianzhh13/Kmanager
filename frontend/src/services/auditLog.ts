import axios from './api'

export interface AuditLogParams {
  page?: number
  page_size?: number
  user_id?: number
  username?: string
  operation?: string
  resource_type?: string
  status?: string
  start_time?: string
  end_time?: string
}

export interface AuditLog {
  id: number
  user_id: number
  username: string
  action: string
  resource: string
  resource_id: string
  cluster_id?: number
  details: string
  ip_address: string
  user_agent: string
  status: string
  error_msg?: string
  created_at: string
}

export interface AuditLogResponse {
  data: AuditLog[]
  total: number
  page: number
  size: number
}

export interface ExportParams extends AuditLogParams {
  format: 'csv' | 'json'
}

const auditLogAPI = {
  // 获取审计日志列表
  list: (params: AuditLogParams) => 
    axios.get<AuditLogResponse>('/audit-logs', { params }),
  
  // 获取单条审计日志
  get: (id: number) => 
    axios.get<AuditLog>(`/audit-logs/${id}`),
  
  // 导出审计日志
  export: (params: ExportParams) => 
    axios.get<string>('/audit-logs/export', { params }),
  
  // 清理过期日志
  clean: (days: number) => 
    axios.post('/audit-logs/clean', null, { params: { days } })
}

export default auditLogAPI