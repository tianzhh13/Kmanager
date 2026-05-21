import { useState, useCallback, useEffect } from 'react'
import { Button, Space, message } from 'antd'
import { EditOutlined, SaveOutlined, UndoOutlined } from '@ant-design/icons'
import { ResponsiveGridLayout, useContainerWidth } from 'react-grid-layout'
import 'react-grid-layout/css/styles.css'
import 'react-resizable/css/styles.css'

// WidthProvider 已在 v2.x 中被移除，改用 useContainerWidth hook

export interface GridItem {
  i: string
  x: number
  y: number
  w: number
  h: number
  minW?: number
  minH?: number
  component: React.ReactNode
}

interface DashboardGridProps {
  items: GridItem[]
  storageKey: string
  cols?: { lg: number; md: number; sm: number; xs: number }
  rowHeight?: number
}

const DEFAULT_COLS = { lg: 12, md: 12, sm: 6, xs: 4 }
const DEFAULT_ROW_HEIGHT = 30

export const DashboardGrid: React.FC<DashboardGridProps> = ({
  items,
  storageKey,
  cols = DEFAULT_COLS,
  rowHeight = DEFAULT_ROW_HEIGHT,
}) => {
  const [isEditing, setIsEditing] = useState(false)
  const [layouts, setLayouts] = useState<any>({})
  const [currentLayout, setCurrentLayout] = useState<any[]>([])
  const { containerRef, width } = useContainerWidth({ initialWidth: 1280 })

  // 从 localStorage 加载布局
  const loadLayout = useCallback(() => {
    try {
      const saved = localStorage.getItem(`dashboard_layout_${storageKey}`)
      if (saved) {
        const parsed = JSON.parse(saved)
        setLayouts(parsed)
        return parsed
      }
    } catch (e) {
      console.error('Failed to load layout:', e)
    }
    return null
  }, [storageKey])

  // 保存布局到 localStorage
  const saveLayout = useCallback((layout: any[]) => {
    try {
      const layoutMap: any = {}
      layout.forEach((item: any) => {
        layoutMap[item.i] = { x: item.x, y: item.y, w: item.w, h: item.h }
      })
      localStorage.setItem(`dashboard_layout_${storageKey}`, JSON.stringify({ lg: layoutMap }))
      message.success('布局已保存')
      setIsEditing(false)
    } catch (e) {
      console.error('Failed to save layout:', e)
      message.error('保存布局失败')
    }
  }, [storageKey])

  // 重置为默认布局
  const resetLayout = useCallback(() => {
    localStorage.removeItem(`dashboard_layout_${storageKey}`)
    setLayouts({})
    setCurrentLayout([])
    message.info('布局已重置')
    setIsEditing(false)
  }, [storageKey])

  // 初始化布局
  useEffect(() => {
    const saved = loadLayout()
    if (!saved) {
      // 使用默认布局
      const defaultLayout = items.map((item) => ({
        i: item.i,
        x: item.x,
        y: item.y,
        w: item.w,
        h: item.h,
        minW: item.minW || 2,
        minH: item.minH || 2,
      }))
      setCurrentLayout(defaultLayout)
    }
  }, [items, loadLayout])

  // 处理布局变化
  const onLayoutChange = useCallback((layout: any) => {
    setCurrentLayout(Array.from(layout))
  }, [])

  // 合并 items 和 layout
  const getLayoutItems = () => {
    const layoutMap: any = layouts.lg || {}
    return items.map(item => {
      const saved = layoutMap[item.i]
      return {
        i: item.i,
        x: saved?.x ?? item.x,
        y: saved?.y ?? item.y,
        w: saved?.w ?? item.w,
        h: saved?.h ?? item.h,
        minW: item.minW || 2,
        minH: item.minH || 2,
      }
    })
  }

  return (
    <div ref={containerRef as any}>
      {/* 编辑模式切换 */}
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'flex-end' }}>
        <Space>
          {isEditing ? (
            <>
              <Button
                type="primary"
                icon={<SaveOutlined />}
                onClick={() => saveLayout(currentLayout)}
              >
                保存布局
              </Button>
              <Button
                icon={<UndoOutlined />}
                onClick={resetLayout}
              >
                重置布局
              </Button>
              <Button onClick={() => setIsEditing(false)}>
                取消
              </Button>
            </>
          ) : (
            <Button
              icon={<EditOutlined />}
              onClick={() => setIsEditing(true)}
            >
              调整布局
            </Button>
          )}
        </Space>
      </div>

      {/* 网格布局 */}
      <ResponsiveGridLayout
        className="layout"
        width={width}
        layouts={{ lg: getLayoutItems() }}
        breakpoints={{ lg: 1200, md: 996, sm: 768, xs: 480 }}
        cols={cols}
        rowHeight={rowHeight}
        onLayoutChange={onLayoutChange as any}
        dragConfig={{ enabled: isEditing, bounded: false, threshold: 3 }}
        resizeConfig={{ enabled: isEditing, handles: ['se'] as any }}
        margin={[16, 16]}
        containerPadding={[0, 0]}
      >
        {items.map(item => (
          <div key={item.i} style={{ overflow: 'auto' }}>
            {item.component}
          </div>
        ))}
      </ResponsiveGridLayout>

      {/* 编辑模式提示 */}
      {isEditing && (
        <div style={{
          position: 'fixed',
          bottom: 24,
          left: '50%',
          transform: 'translateX(-50%)',
          background: '#1890ff',
          color: '#fff',
          padding: '8px 24px',
          borderRadius: 4,
          zIndex: 1000,
          boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
        }}>
          编辑模式：拖拽卡片调整位置，拖拽边缘调整大小
        </div>
      )}
    </div>
  )
}

export default DashboardGrid
