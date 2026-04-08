import { createSlice, createAsyncThunk } from '@reduxjs/toolkit'
import { clusterAPI, Cluster, CreateClusterRequest } from '../../services/cluster'

interface ClusterState {
  clusters: Cluster[]
  currentCluster: Cluster | null
  total: number
  loading: boolean
  error: string | null
}

const initialState: ClusterState = {
  clusters: [],
  currentCluster: null,
  total: 0,
  loading: false,
  error: null,
}

export const fetchClusters = createAsyncThunk(
  'clusters/fetchClusters',
  async (params: { page: number; pageSize: number }, { rejectWithValue }) => {
    try {
      const response = await clusterAPI.list(params.page, params.pageSize)
      return response
    } catch (error: any) {
      return rejectWithValue(error.response?.data?.error || '获取集群列表失败')
    }
  }
)

export const createCluster = createAsyncThunk(
  'clusters/createCluster',
  async (data: CreateClusterRequest, { rejectWithValue }) => {
    try {
      const response = await clusterAPI.create(data)
      return response
    } catch (error: any) {
      return rejectWithValue(error.response?.data?.error || '创建集群失败')
    }
  }
)

export const testClusterConnection = createAsyncThunk(
  'clusters/testConnection',
  async (clusterId: number, { rejectWithValue }) => {
    try {
      await clusterAPI.testConnection(clusterId)
      return { clusterId, success: true }
    } catch (error: any) {
      return rejectWithValue(error.response?.data?.error || '连接测试失败')
    }
  }
)

const clusterSlice = createSlice({
  name: 'clusters',
  initialState,
  reducers: {
    setCurrentCluster: (state, action) => {
      state.currentCluster = action.payload
    },
    clearError: (state) => {
      state.error = null
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchClusters.pending, (state) => {
        state.loading = true
      })
      .addCase(fetchClusters.fulfilled, (state, action) => {
        state.loading = false
        state.clusters = action.payload.data
        state.total = action.payload.total
      })
      .addCase(fetchClusters.rejected, (state, action) => {
        state.loading = false
        state.error = action.payload as string
      })
      .addCase(createCluster.fulfilled, (state, action) => {
        state.clusters.unshift(action.payload)
        state.total += 1
      })
      .addCase(testClusterConnection.fulfilled, (state) => {
        // Connection test passed
      })
  },
})

export const { setCurrentCluster, clearError } = clusterSlice.actions
export default clusterSlice.reducer