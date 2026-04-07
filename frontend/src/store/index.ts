import { configureStore } from '@reduxjs/toolkit'
import authReducer from './slices/authSlice'
import clusterReducer from './slices/clusterSlice'
import topicReducer from './slices/topicSlice'

export const store = configureStore({
  reducer: {
    auth: authReducer,
    clusters: clusterReducer,
    topics: topicReducer,
  },
})

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch