/**
 * ECharts 按需引入 — 只注册实际使用的图表类型和组件
 * 全量 echarts ~1MB，按需引入后 ~200-300KB
 */
import * as echarts from 'echarts/core'

// 图表类型：项目只用了 line、bar、pie
import { LineChart } from 'echarts/charts'
import { BarChart } from 'echarts/charts'
import { PieChart } from 'echarts/charts'

// 组件：grid、tooltip、legend 是基础组件，graphic 用于"暂无数据"文字
import {
  GridComponent,
  TooltipComponent,
  LegendComponent,
  TitleComponent,
  GraphicComponent,
} from 'echarts/components'

// 渲染器
import { CanvasRenderer } from 'echarts/renderers'

// 注册
echarts.use([
  LineChart,
  BarChart,
  PieChart,
  GridComponent,
  TooltipComponent,
  LegendComponent,
  TitleComponent,
  GraphicComponent,
  CanvasRenderer,
])

export default echarts
