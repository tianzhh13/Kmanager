import React from 'react'
import ReactDOM from 'react-dom/client'
import { Provider } from 'react-redux'
import { BrowserRouter } from 'react-router-dom'
import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import App from './App'
import { store } from './store'
import './index.css'

// Brand theme tokens — Sky Blue + Coral Accent
const theme = {
  token: {
    // Primary color — Sky Blue
    colorPrimary: '#0ea5e9',
    colorPrimaryHover: '#0284c7',
    colorPrimaryActive: '#0369a1',
    colorPrimaryBg: 'rgba(14, 165, 233, 0.06)',
    colorPrimaryBorder: 'rgba(14, 165, 233, 0.20)',

    // Text
    colorText: '#1e293b',
    colorTextSecondary: '#64748b',
    colorTextTertiary: '#94a3b8',
    colorTextHeading: '#0f172a',

    // Background
    colorBgContainer: '#ffffff',
    colorBgLayout: '#fafbfc',
    colorBgElevated: '#ffffff',

    // Border
    colorBorder: '#e5eaf0',
    colorBorderSecondary: '#f1f5f9',

    // Radius
    borderRadius: 8,
    borderRadiusSM: 6,
    borderRadiusLG: 12,

    // Font
    fontFamily: "'Plus Jakarta Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', sans-serif",
    fontSize: 14,
    fontSizeSM: 12,
    fontSizeLG: 16,

    // Shadow
    boxShadow: '0 1px 3px rgba(0, 0, 0, 0.03), 0 1px 2px rgba(0, 0, 0, 0.02)',
    boxShadowSecondary: '0 8px 24px rgba(14, 165, 233, 0.07), 0 2px 8px rgba(0, 0, 0, 0.03)',

    // Motion
    motionDurationFast: '0.15s',
    motionDurationMid: '0.25s',

    // Link
    colorLink: '#0ea5e9',
    colorLinkHover: '#0284c7',
    colorLinkActive: '#0369a1',
  },
  components: {
    Table: {
      headerBg: '#fafbfc',
      headerColor: '#64748b',
      headerSortActiveBg: '#f1f5f9',
      rowHoverBg: 'rgba(14, 165, 233, 0.04)',
      borderColor: '#f1f5f9',
    },
    Card: {
      paddingLG: 20,
    },
    Button: {
      primaryShadow: '0 1px 3px rgba(14, 165, 233, 0.25)',
    },
    Menu: {
      itemBorderRadius: 8,
      itemMarginInline: 0,
      itemHeight: 40,
    },
    Modal: {
      borderRadiusLG: 16,
    },
    Select: {
      optionSelectedBg: 'rgba(14, 165, 233, 0.06)',
    },
    Tag: {
      borderRadiusSM: 6,
    },
    Statistic: {
      titleFontSize: 12,
      contentFontSize: 20,
    },
  },
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <Provider store={store}>
      <BrowserRouter>
        <ConfigProvider locale={zhCN} theme={theme}>
          <App />
        </ConfigProvider>
      </BrowserRouter>
    </Provider>
  </React.StrictMode>,
)
