/**
 * 通用 ECharts 图表配置工厂函数 — Bento Design System
 * 统一品牌色 + 渐变填充 + JetBrains Mono 标签 + 虚线网格 + 圆角 tooltip
 */

/** 格式化字节用于图表显示 */
export const formatBytesForChart = (bytes: number): string => {
  if (!bytes || bytes === 0) return '0 B'
  if (bytes >= 1024 * 1024 * 1024) return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB'
  if (bytes >= 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
  if (bytes >= 1024) return (bytes / 1024).toFixed(2) + ' KB'
  return bytes.toFixed(0) + ' B'
}

// ---- Brand Color Palette (replaces Ant Design defaults) ----

export const BRAND_COLORS = [
  '#f97316', '#fb923c', '#fdba74', '#fed7aa', '#ffedd5',
  '#0d9488', '#2dd4bf', '#5eead4', '#99f6e4',
  '#7c3aed', '#a78bfa', '#c4b5fd',
]

/** 多 series 配色 */
const MULTI_SERIES_COLORS = BRAND_COLORS

// ---- Shared Axis / Grid / Tooltip style fragments ----

const SHARED_AXIS_STYLE = {
  axisLine: { show: false },
  axisTick: { show: false },
  axisLabel: {
    fontFamily: "'JetBrains Mono', monospace",
    fontSize: 11,
    color: '#a8a29e',
  },
  splitLine: {
    lineStyle: {
      type: 'dashed' as const,
      color: '#ebe8e3',
    },
  },
}

const SHARED_X_AXIS = {
  ...SHARED_AXIS_STYLE,
  type: 'category' as const,
  boundaryGap: false,
}

const SHARED_Y_AXIS = {
  ...SHARED_AXIS_STYLE,
  type: 'value' as const,
}

const SHARED_TOOLTIP = {
  trigger: 'axis' as const,
  backgroundColor: '#fff',
  borderColor: '#ebe8e3',
  borderWidth: 1,
  borderRadius: 10,
  padding: [10, 14],
  textStyle: {
    fontFamily: "'Plus Jakarta Sans', sans-serif",
    fontSize: 12,
    color: '#1c1917',
  },
  extraCssText: 'box-shadow: 0 6px 20px rgba(0,0,0,0.08);',
}

const SHARED_GRID = {
  left: '3%',
  right: '4%',
  bottom: '10%',
  top: '12%',
  containLabel: true,
}

const ROUNDED_LINE_STYLE = {
  smooth: true,
  symbol: 'circle',
  symbolSize: 5,
  showSymbol: false,
  connectNulls: true,
  lineStyle: { width: 2.5 },
}

// ---- Empty state helper ----

function emptyOption(title?: string, emptyText = '暂无数据'): Record<string, any> {
  return {
    title: title ? {
      text: title,
      left: 16,
      top: 8,
      textStyle: {
        fontFamily: "'DM Sans', sans-serif",
        fontSize: 13,
        fontWeight: 600,
        color: '#44403c',
      },
    } : { show: false },
    graphic: {
      type: 'text',
      left: 'center',
      top: 'middle',
      style: {
        text: emptyText,
        fill: '#a8a29e',
        fontSize: 14,
        fontFamily: "'Plus Jakarta Sans', sans-serif",
      },
    },
    xAxis: { type: 'category', data: [], show: false },
    yAxis: { type: 'value', show: false },
    series: [],
  }
}

// ---- Factory: Area Chart (primary chart type) ----

export interface AreaChartData {
  times: string[]
  values: (number | null)[]
}

export function createAreaChartOption(
  title: string,
  data: AreaChartData,
  color: string = BRAND_COLORS[0],
  unit: string = '',
  formatter?: (value: number) => string,
): Record<string, any> {
  const hasData = data.times.length > 0 && data.values.some(v => v !== null && v !== undefined)
  if (!hasData) return emptyOption(title)

  const tooltipFormatter = (params: any) => {
    if (!params || params.length === 0) return ''
    const val = formatter
      ? formatter(params[0].value)
      : (params[0].value?.toFixed(2) ?? '0')
    const unitStr = unit ? ` ${unit}` : ''
    return `<span style="font-weight:600;font-size:13px;">${params[0].axisValue}</span><br/><span style="color:${color};font-weight:700;">●</span> ${val}${unitStr}`
  }

  return {
    title: {
      text: title,
      left: 16,
      top: 8,
      textStyle: {
        fontFamily: "'DM Sans', sans-serif",
        fontSize: 13,
        fontWeight: 600,
        color: '#44403c',
      },
    },
    tooltip: {
      ...SHARED_TOOLTIP,
      formatter: tooltipFormatter,
    },
    grid: SHARED_GRID,
    xAxis: { ...SHARED_X_AXIS, data: data.times },
    yAxis: {
      ...SHARED_Y_AXIS,
      name: unit || undefined,
      nameTextStyle: { fontFamily: "'JetBrains Mono', monospace", fontSize: 11, color: '#a8a29e', padding: [0, 0, 0, -20] },
      axisLabel: formatter ? { ...SHARED_AXIS_STYLE.axisLabel, formatter } : SHARED_AXIS_STYLE.axisLabel,
    },
    series: [{
      ...ROUNDED_LINE_STYLE,
      type: 'line',
      data: data.values,
      itemStyle: { color },
      areaStyle: {
        color: {
          type: 'linear',
          x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: color + '28' },
            { offset: 1, color: color + '05' },
          ],
        },
      },
    }],
  }
}

// ---- Factory: Multi-Line Chart (for merged broker/partition metrics) ----

export interface MultiLineSeries {
  name: string
  times: string[]
  values: number[]
}

export function createMultiLineChartOption(
  title: string,
  seriesList: MultiLineSeries[],
  yAxisName: string = '',
  tooltipFormatter?: (value: number) => string,
): Record<string, any> {
  if (!seriesList || seriesList.length === 0) return emptyOption(title)

  // Merge all time labels
  const allTimes = new Set<string>()
  seriesList.forEach(s => s.times.forEach(t => allTimes.add(t)))
  const times = Array.from(allTimes).sort()

  const buildTooltip = (params: any[]) => {
    if (!params || params.length === 0) return ''
    let html = `<span style="font-weight:600;font-size:13px;">${params[0].axisValue}</span><br/>`
    params.filter(p => p.value !== undefined && p.value !== null).forEach(p => {
      const val = tooltipFormatter ? tooltipFormatter(p.value) : (p.value?.toFixed(2) ?? '0')
      html += `${p.marker} ${p.seriesName}: <span style="font-weight:600;">${val}</span><br/>`
    })
    return html
  }

  return {
    title: {
      text: title,
      left: 16,
      top: 8,
      textStyle: {
        fontFamily: "'DM Sans', sans-serif",
        fontSize: 13,
        fontWeight: 600,
        color: '#44403c',
      },
    },
    tooltip: {
      ...SHARED_TOOLTIP,
      formatter: buildTooltip,
    },
    legend: {
      data: seriesList.map(s => s.name),
      top: 0,
      right: 0,
      type: 'scroll',
      icon: 'roundRect',
      itemWidth: 14,
      itemHeight: 3,
      textStyle: { fontFamily: "'Plus Jakarta Sans', sans-serif", fontSize: 11, color: '#57534e' },
    },
    grid: { ...SHARED_GRID, top: 36 },
    xAxis: { ...SHARED_X_AXIS, data: times },
    yAxis: {
      ...SHARED_Y_AXIS,
      name: yAxisName || undefined,
      nameTextStyle: { fontFamily: "'JetBrains Mono', monospace", fontSize: 11, color: '#a8a29e', padding: [0, 0, 0, -20] },
    },
    series: seriesList.map((s, idx) => ({
      ...ROUNDED_LINE_STYLE,
      name: s.name,
      type: 'line',
      data: times.map(t => {
        const ti = s.times.indexOf(t)
        return ti >= 0 ? s.values[ti] : null
      }),
      itemStyle: { color: MULTI_SERIES_COLORS[idx % MULTI_SERIES_COLORS.length] },
      areaStyle: {
        color: {
          type: 'linear',
          x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: MULTI_SERIES_COLORS[idx % MULTI_SERIES_COLORS.length] + '18' },
            { offset: 1, color: MULTI_SERIES_COLORS[idx % MULTI_SERIES_COLORS.length] + '03' },
          ],
        },
      },
    })),
  }
}

// ---- Factory: Donut / Ring Chart ----

export interface DonutDataItem {
  name: string
  value: number
  color?: string
}

export function createDonutChartOption(
  data: DonutDataItem[],
  radius: [string, string] = ['55%', '78%'],
): Record<string, any> {
  if (!data || data.length === 0) return emptyOption('')

  return {
    title: { show: false },
    tooltip: {
      ...SHARED_TOOLTIP,
      trigger: 'item',
      formatter: (params: any) => {
        return `<span style="font-weight:600;">${params.name}</span><br/><span style="color:${params.color};font-weight:700;">●</span> ${params.value} (${params.percent}%)`
      },
    },
    series: [{
      type: 'pie',
      radius,
      center: ['50%', '50%'],
      avoidLabelOverlap: false,
      itemStyle: {
        borderRadius: 6,
        borderColor: '#fff',
        borderWidth: 2,
      },
      label: { show: false },
      emphasis: {
        label: { show: false },
        itemStyle: {
          shadowBlur: 10,
          shadowOffsetX: 0,
          shadowColor: 'rgba(0,0,0,0.1)',
        },
      },
      data: data.map((d, idx) => ({
        name: d.name,
        value: d.value,
        itemStyle: { color: d.color || BRAND_COLORS[idx % BRAND_COLORS.length] },
      })),
    }],
  }
}

// ---- Factory: Stacked Bar Chart ----

export interface StackedBarSeries {
  name: string
  data: number[]
  color?: string
}

export function createStackedBarChartOption(
  categories: string[],
  seriesList: StackedBarSeries[],
  yAxisName: string = '',
): Record<string, any> {
  if (!seriesList || seriesList.length === 0) return emptyOption('')

  return {
    title: { show: false },
    tooltip: { ...SHARED_TOOLTIP },
    legend: {
      data: seriesList.map(s => s.name),
      top: 0,
      right: 0,
      icon: 'roundRect',
      itemWidth: 14,
      itemHeight: 3,
      textStyle: { fontFamily: "'Plus Jakarta Sans', sans-serif", fontSize: 11, color: '#57534e' },
    },
    grid: { ...SHARED_GRID, top: 36 },
    xAxis: { ...SHARED_X_AXIS, data: categories, boundaryGap: true },
    yAxis: {
      ...SHARED_Y_AXIS,
      name: yAxisName || undefined,
    },
    series: seriesList.map((s, idx) => ({
      name: s.name,
      type: 'bar',
      stack: 'total',
      barMaxWidth: 32,
      itemStyle: {
        color: s.color || MULTI_SERIES_COLORS[idx % MULTI_SERIES_COLORS.length],
        borderRadius: idx === seriesList.length - 1 ? [4, 4, 0, 0] : [0, 0, 0, 0],
      },
      data: s.data,
    })),
  }
}

// ---- Factory: Horizontal Stacked Bar Chart ----

export function createHorizontalStackedBarChartOption(
  categories: string[],
  seriesList: StackedBarSeries[],
  xAxisName: string = '',
): Record<string, any> {
  if (!seriesList || seriesList.length === 0) return emptyOption('')

  return {
    title: { show: false },
    tooltip: { ...SHARED_TOOLTIP },
    legend: {
      data: seriesList.map(s => s.name),
      top: 0,
      right: 0,
      icon: 'roundRect',
      itemWidth: 14,
      itemHeight: 3,
      textStyle: { fontFamily: "'Plus Jakarta Sans', sans-serif", fontSize: 11, color: '#57534e' },
    },
    grid: { ...SHARED_GRID, top: 36 },
    yAxis: {
      ...SHARED_AXIS_STYLE,
      type: 'category' as const,
      data: categories,
      axisLabel: {
        ...SHARED_AXIS_STYLE.axisLabel,
        width: 120,
        overflow: 'truncate',
      },
    },
    xAxis: {
      ...SHARED_AXIS_STYLE,
      type: 'value' as const,
      name: xAxisName || undefined,
      nameTextStyle: { fontFamily: "'JetBrains Mono', monospace", fontSize: 11, color: '#a8a29e', padding: [0, 0, 0, -20] },
    },
    series: seriesList.map((s, idx) => ({
      name: s.name,
      type: 'bar',
      stack: 'total',
      barMaxWidth: 24,
      itemStyle: {
        color: s.color || MULTI_SERIES_COLORS[idx % MULTI_SERIES_COLORS.length],
        borderRadius: idx === seriesList.length - 1 ? [0, 4, 4, 0] : [0, 0, 0, 0],
      },
      data: s.data,
    })),
  }
}

// ---- Factory: Grouped Bar Chart ----

export interface GroupedBarSeries {
  name: string
  data: number[]
  color?: string
}

export function createGroupedBarChartOption(
  categories: string[],
  seriesList: GroupedBarSeries[],
  yAxisName: string = '',
): Record<string, any> {
  if (!seriesList || seriesList.length === 0) return emptyOption('')

  return {
    title: { show: false },
    tooltip: { ...SHARED_TOOLTIP },
    legend: {
      data: seriesList.map(s => s.name),
      top: 0,
      right: 0,
      icon: 'roundRect',
      itemWidth: 14,
      itemHeight: 3,
      textStyle: { fontFamily: "'Plus Jakarta Sans', sans-serif", fontSize: 11, color: '#57534e' },
    },
    grid: { ...SHARED_GRID, top: 36 },
    xAxis: { ...SHARED_X_AXIS, data: categories, boundaryGap: true },
    yAxis: {
      ...SHARED_Y_AXIS,
      name: yAxisName || undefined,
    },
    series: seriesList.map((s, idx) => ({
      name: s.name,
      type: 'bar',
      barMaxWidth: 24,
      barGap: '20%',
      itemStyle: {
        color: s.color || MULTI_SERIES_COLORS[idx % MULTI_SERIES_COLORS.length],
        borderRadius: [4, 4, 0, 0],
      },
      data: s.data,
    })),
  }
}

// ============================================================
// Legacy compatibility wrappers (kept for gradual migration)
// These delegate to the new Bento factories with same signatures
// ============================================================

/**
 * @deprecated Use createAreaChartOption instead
 */
export const buildLineChartOption = (
  title: string,
  data: { times: string[]; values: (number | null)[] },
  color: string = '#f97316',
  unit: string = '',
  formatter?: (value: number) => string,
): Record<string, any> => {
  return createAreaChartOption(title, data, color, unit, formatter)
}

/**
 * @deprecated Use createStackedBarChartOption or createGroupedBarChartOption instead
 */
export const buildBarChartOption = (
  _title: string,
  categories: string[],
  seriesList: Array<{ name: string; data: number[]; color: string }>,
  yAxisName: string = '',
): Record<string, any> => {
  return createGroupedBarChartOption(categories, seriesList, yAxisName)
}

/**
 * @deprecated Use createMultiLineChartOption instead
 */
export const buildMultiSeriesChartOption = (
  title: string,
  data: Record<string, { times: string[]; values: number[] }> | { times: string[]; values: number[] } | undefined,
  _color: string,
  yAxisName: string,
  tooltipFormatter?: (value: number) => string,
  fullTimes?: string[],
): Record<string, any> => {
  let safeData: Record<string, { times: string[]; values: number[] }> = {}
  if (data && typeof data === 'object') {
    if ('times' in data && 'values' in data) {
      safeData = { '0': data as { times: string[]; values: number[] } }
    } else {
      safeData = data as Record<string, { times: string[]; values: number[] }>
    }
  }
  const entries = Object.entries(safeData).filter(([, d]) => d && d.times && d.values)
  if (entries.length === 0) return emptyOption(title)

  const seriesList: MultiLineSeries[] = entries.map(([id, d]) => ({
    name: `Broker ${id}`,
    times: fullTimes || d.times,
    values: d.times.map((t, i) => {
      if (fullTimes) {
        const idx = d.times.indexOf(t)
        return idx >= 0 ? d.values[idx] : 0
      }
      return d.values[i]
    }),
  }))

  return createMultiLineChartOption(title, seriesList, yAxisName, tooltipFormatter)
}

/**
 * @deprecated Use createMultiLineChartOption instead
 */
export const buildPartitionChartOption = (
  title: string,
  metrics: Array<{ partition: number; values: { time: string; value: number }[] }>,
  selectedPartitions: number[],
  yAxisName: string,
  tooltipFormatter?: (value: number) => string,
  emptyText: string = '暂无数据',
): Record<string, any> => {
  const filteredMetrics = metrics.filter(p => selectedPartitions.includes(p.partition) && p.values.length > 0)
  if (filteredMetrics.length === 0) return emptyOption(title, emptyText)

  const seriesList: MultiLineSeries[] = filteredMetrics.map(p => ({
    name: `分区${p.partition}`,
    times: p.values.map(v => v.time),
    values: p.values.map(v => v.value),
  }))

  return createMultiLineChartOption(title, seriesList, yAxisName, tooltipFormatter)
}
