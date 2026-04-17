import { createSlice, createAsyncThunk } from '@reduxjs/toolkit'
import { topicService, Topic, CreateTopicRequest } from '../../services/topic'

interface TopicState {
  topics: Topic[]
  currentTopic: Topic | null
  total: number
  loading: boolean
  error: string | null
}

const initialState: TopicState = {
  topics: [],
  currentTopic: null,
  total: 0,
  loading: false,
  error: null,
}

export const fetchTopics = createAsyncThunk(
  'topics/fetchTopics',
  async (params: { page: number; pageSize: number; clusterId?: number }, { rejectWithValue }) => {
    try {
      const response = await topicService.list(params.page, params.pageSize, params.clusterId)
      return response
    } catch (error: any) {
      return rejectWithValue(error.response?.data?.error || '获取 Topic 列表失败')
    }
  }
)

export const createTopic = createAsyncThunk(
  'topics/createTopic',
  async (data: CreateTopicRequest, { rejectWithValue }) => {
    try {
      const response = await topicService.create(data)
      return response
    } catch (error: any) {
      return rejectWithValue(error.response?.data?.error || '创建 Topic 失败')
    }
  }
)

export const deleteTopic = createAsyncThunk(
  'topics/deleteTopic',
  async (params: { topicName: string; clusterId: number }, { rejectWithValue }) => {
    try {
      await topicService.delete(params.topicName, params.clusterId)
      return params.topicName
    } catch (error: any) {
      return rejectWithValue(error.response?.data?.error || '删除 Topic 失败')
    }
  }
)

const topicSlice = createSlice({
  name: 'topics',
  initialState,
  reducers: {
    setCurrentTopic: (state, action) => {
      state.currentTopic = action.payload
    },
    clearError: (state) => {
      state.error = null
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchTopics.pending, (state) => {
        state.loading = true
      })
      .addCase(fetchTopics.fulfilled, (state, action) => {
        state.loading = false
        state.topics = action.payload.data
        state.total = action.payload.total
      })
      .addCase(fetchTopics.rejected, (state, action) => {
        state.loading = false
        state.error = action.payload as string
      })
      .addCase(createTopic.fulfilled, (_state) => {
        // 创建成功后由页面重新获取列表，这里不需要处理
      })
      .addCase(deleteTopic.fulfilled, (state, action) => {
        state.topics = state.topics.filter(t => t.topic_name !== action.payload)
        state.total -= 1
      })
  },
})

export const { setCurrentTopic, clearError } = topicSlice.actions
export default topicSlice.reducer