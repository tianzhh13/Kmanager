import request from './api'

export interface ACL {
  id: number
  cluster_id: number
  resource_type: string
  resource_name: string
  resource_pattern: string
  principal: string
  host: string
  operation: string
  permission_type: string
  created_at: string
}

export interface UserACLInfo {
  resource_type: string
  resource_name: string
  resource_pattern: string
  principal: string
  host: string
  operation: string
  permission_type: string
}

export interface CreateACLRequest {
  cluster_id: number
  resource_type: string
  resource_name: string
  principal: string
  operation: string
  permission_type: string
  resource_pattern?: string
  host?: string
}

export interface DeleteACLFromKafkaRequest {
  resource_type: string
  resource_name: string
  resource_pattern: string
  principal: string
  host: string
  operation: string
  permission_type: string
}

export const getACLs = (params?: { cluster_id?: number; resource_type?: string; principal?: string }) => {
  return request.get('/acls', { params }).then(res => {
    return res.data?.data || res.data || []
  })
}

export const getUserACLsFromKafka = (clusterId: number, principal: string) => {
  return request.get('/acls/user', {
    params: { cluster_id: clusterId, principal }
  }).then(res => {
    return res.data?.data || []
  })
}

export const createACL = (data: CreateACLRequest) => {
  return request.post('/acls', data).then(res => {
    return res.data
  })
}

export const deleteACL = (id: number, clusterId: number) => {
  return request.delete(`/acls/${id}`, {
    params: { cluster_id: clusterId }
  }).then(res => {
    return res.data
  })
}

export const deleteACLFromKafka = (clusterId: number, data: DeleteACLFromKafkaRequest) => {
  return request.delete('/acls/kafka', {
    params: { cluster_id: clusterId },
    data
  }).then(res => {
    return res.data
  })
}

export const batchDeleteACL = (ids: number[]) => {
  return request.post('/acls/batch-delete', { ids })
}

export const syncACLs = (clusterId: number) => {
  return request.post(`/acls/sync/${clusterId}`).then(res => {
    return res.data
  })
}
