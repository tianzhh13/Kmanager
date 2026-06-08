import api from './api'

export interface HostMapping {
  id: number
  hostname: string
  ip_address: string
  description: string
  created_at: string
  updated_at: string
}

export interface CreateHostMappingRequest {
  hostname: string
  ip_address: string
  description?: string
}

export interface UpdateHostMappingRequest {
  hostname?: string
  ip_address?: string
  description?: string
}

export interface HostMappingListResponse {
  data: HostMapping[]
  total: number
  page: number
  page_size: number
}

export const hostMappingAPI = {
  list: async (page: number = 1, pageSize: number = 20, keyword?: string): Promise<HostMappingListResponse> => {
    const response = await api.get('/host-mappings', {
      params: { page, page_size: pageSize, keyword: keyword || undefined },
    })
    return response.data
  },

  get: async (id: number): Promise<HostMapping> => {
    const response = await api.get(`/host-mappings/${id}`)
    return response.data
  },

  create: async (data: CreateHostMappingRequest): Promise<HostMapping> => {
    const response = await api.post('/host-mappings', data)
    return response.data
  },

  update: async (id: number, data: UpdateHostMappingRequest): Promise<HostMapping> => {
    const response = await api.put(`/host-mappings/${id}`, data)
    return response.data
  },

  delete: async (id: number): Promise<void> => {
    await api.delete(`/host-mappings/${id}`)
  },
}
