import request from './api'

export interface ACL {
  id: number
  cluster_id: number
  resource_type: string
  resource_name: string
  principal: string
  operation: string
  permission: string
  created_at: string
}

export interface CreateACLRequest {
  cluster_id: number
  resource_type: string
  resource_name: string
  principal: string
  operation: string
  permission: string
}

export const getACLs = (params?: { cluster_id?: number; resource_type?: string; principal?: string }) => {
  return request.get<{ data: ACL[] }>('/acls', { params }).then(res => res.data.data)
}

export const createACL = (data: CreateACLRequest) => {
  return request.post('/acls', data)
}

export const deleteACL = (id: number) => {
  return request.delete(`/acls/${id}`)
}

export const batchDeleteACL = (ids: number[]) => {
  return request.post('/acls/batch-delete', { ids })
}