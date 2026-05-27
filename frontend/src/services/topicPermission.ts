import api from './api'

export interface TopicPermission {
  id: number
  user_id: number
  username: string
  cluster_id: number
  cluster_name: string
  topic_name: string
  created_at: string
  created_by: number
}

export interface AssignTopicPermissionRequest {
  user_id: number
  cluster_id: number
  topic_name: string
}

export interface BatchAssignTopicPermissionRequest {
  user_id: number
  cluster_id: number
  topic_names: string[]
}

export interface RevokeTopicPermissionRequest {
  user_id: number
  cluster_id: number
  topic_name: string
}

export const topicPermissionService = {
  // 分配单个 Topic 权限
  assign: async (data: AssignTopicPermissionRequest) => {
    const response = await api.post('/topic-permissions', data)
    return response.data
  },

  // 批量分配 Topic 权限
  batchAssign: async (data: BatchAssignTopicPermissionRequest) => {
    const response = await api.post('/topic-permissions/batch', data)
    return response.data
  },

  // 撤销 Topic 权限
  revoke: async (data: RevokeTopicPermissionRequest) => {
    const response = await api.delete('/topic-permissions', { data })
    return response.data
  },

  // 获取用户的 Topic 权限列表
  getUserPermissions: async (userId: number): Promise<{ data: TopicPermission[] }> => {
    const response = await api.get(`/topic-permissions/user/${userId}`)
    return response.data
  },

  // 获取用户在指定集群的 Topic 权限列表
  getUserClusterPermissions: async (userId: number, clusterId: number): Promise<{ data: string[] }> => {
    const response = await api.get(`/topic-permissions/user/${userId}/cluster/${clusterId}`)
    return response.data
  },
}
