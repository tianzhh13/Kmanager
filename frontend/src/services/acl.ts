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
  console.log('=== DEBUG: getACLs called with params ===', params)
  return request.get('/acls', { params }).then(res => {
    console.log('=== DEBUG: getACLs response ===', res)
    return res.data?.data || res.data || []
  })
}

export const getUserACLsFromKafka = (clusterId: number, principal: string) => {
  console.log('=== DEBUG: getUserACLsFromKafka called, clusterId ===', clusterId, 'principal ===', principal)
  return request.get('/acls/user', {
    params: { cluster_id: clusterId, principal }
  }).then(res => {
    console.log('=== DEBUG: getUserACLsFromKafka response ===', res)
    return res.data?.data || []
  })
}

export const createACL = (data: CreateACLRequest) => {
  console.log('=== DEBUG: createACL called with ===', data)
  return request.post('/acls', data).then(res => {
    console.log('=== DEBUG: createACL response ===', res)
    return res.data
  })
}

export const deleteACL = (id: number, clusterId: number) => {
  console.log('=== DEBUG: deleteACL called, id ===', id, 'clusterId ===', clusterId)
  return request.delete(`/acls/${id}`, {
    params: { cluster_id: clusterId }
  }).then(res => {
    console.log('=== DEBUG: deleteACL response ===', res)
    return res.data
  })
}

export const deleteACLFromKafka = (clusterId: number, data: DeleteACLFromKafkaRequest) => {
  console.log('=== DEBUG: deleteACLFromKafka called, clusterId ===', clusterId, 'data ===', data)
  return request.delete('/acls/kafka', {
    params: { cluster_id: clusterId },
    data
  }).then(res => {
    console.log('=== DEBUG: deleteACLFromKafka response ===', res)
    return res.data
  })
}

export const batchDeleteACL = (ids: number[]) => {
  console.log('=== DEBUG: batchDeleteACL called with ===', ids)
  return request.post('/acls/batch-delete', { ids })
}

export const syncACLs = (clusterId: number) => {
  console.log('=== DEBUG: syncACLs called, clusterId ===', clusterId)
  return request.post(`/acls/sync/${clusterId}`).then(res => {
    console.log('=== DEBUG: syncACLs response ===', res)
    return res.data
  })
}
