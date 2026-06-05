import React from 'react'
import ReactDOM from 'react-dom/client'
import { Provider } from 'react-redux'
import { BrowserRouter } from 'react-router-dom'
import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import App from './App'
import { store } from './store'
import './index.css'
import './components/bento/styles.css'

// Brand theme tokens — Coral Accent + Stone Neutral
const theme = {
  token: {
    // Primary color — Coral
    colorPrimary: '#f97316',
    colorPrimaryHover: '#ea580c',
    colorPrimaryActive: '#c2410c',
    colorPrimaryBg: 'rgba(249, 115, 22, 0.06)',
    colorPrimaryBorder: 'rgba(249, 115, 22, 0.20)',

    // Text — Stone (warm gray)
    colorText: '#1c1917',
    colorTextSecondary: '#57534e',
    colorTextTertiary: '#a8a29e',
    colorTextHeading: '#0c0a09',

    // Background
    colorBgContainer: '#ffffff',
    colorBgLayout: '#faf9f7',
    colorBgElevated: '#ffffff',

    // Border — warm gray
    colorBorder: '#ebe8e3',
    colorBorderSecondary: '#f3f1ee',

    // Radius
    borderRadius: 8,
    borderRadiusSM: 6,
    borderRadiusLG: 12,

    // Font
    fontFamily: "'Plus Jakarta Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', sans-serif",
    fontSize: 14,
    fontSizeSM: 12,
    fontSizeLG: 16,

    // Shadow — warm tinted
    boxShadow: '0 1px 3px rgba(120, 80, 40, 0.04), 0 1px 2px rgba(120, 80, 40, 0.03)',
    boxShadowSecondary: '0 8px 24px rgba(249, 115, 22, 0.07), 0 2px 8px rgba(120, 80, 40, 0.04)',

    // Motion — spring physics
    motionDurationFast: '0.18s',
    motionDurationMid: '0.32s',

    // Link
    colorLink: '#f97316',
    colorLinkHover: '#ea580c',
    colorLinkActive: '#c2410c',
  },
  components: {
    Table: {
      headerBg: '#faf9f7',
      headerColor: '#57534e',
      headerSortActiveBg: '#f3f1ee',
      rowHoverBg: 'rgba(249, 115, 22, 0.04)',
      borderColor: '#f3f1ee',
    },
    Card: {
      paddingLG: 20,
    },
    Button: {
      primaryShadow: '0 1px 3px rgba(249, 115, 22, 0.25)',
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
      optionSelectedBg: 'rgba(249, 115, 22, 0.06)',
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
