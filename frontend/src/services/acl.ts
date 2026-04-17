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
  permission_type: string
  resource_pattern?: string
  host?: string
}

export const getACLs = (params?: { cluster_id?: number; resource_type?: string; principal?: string }) => {
  console.log('=== DEBUG: getACLs called with params ===', params)
  return request.get('/acls', { params }).then(res => {
    console.log('=== DEBUG: getACLs response ===', res)
    // 后端返回格式: { data: [...], total: ... }
    return res.data?.data || res.data || []
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
