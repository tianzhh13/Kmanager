/**
 * 通用 ECharts 图表配置工厂函数
 * 统一 tooltip、axis、legend 样式，消除 Monitor.tsx 中的重复图表函数
 */

/** 格式化字节用于图表显示 */
export const formatBytesForChart = (bytes: number): string => {
  if (!bytes || bytes === 0) return '0 B'
  if (bytes >= 1024 * 1024 * 1024) return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB'
  if (bytes >= 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
  if (bytes >= 1024) return (bytes / 1024).toFixed(2) + ' KB'
  return bytes.toFixed(0) + ' B'
}

/** 多 series 折线图默认配色 */
const MULTI_SERIES_COLORS = ['#1890ff', '#52c41a', '#faad14', '#f5222d', '#722ed1', '#13c2c2', '#eb2f96', '#fa8c16']

/**
 * 构建通用单 series 折线图配置
 */
export const buildLineChartOption = (
  title: string,
  data: { times: string[]; values: (number | null)[] },
  color: string = '#1890ff',
  unit: string = '',
  formatter?: (value: number) => string,
): Record<string, any> => {
  const hasData = data.times.length > 0 && data.values.length > 0

  if (!hasData) {
    return {
      title: { text: title, left: 'center', textStyle: { fontSize: 14, color: '#999' } },
      graphic: { type: 'text', left: 'center', top: 'middle', style: { text: '暂无数据', fill: '#999', fontSize: 14 } },
      xAxis: { type: 'category', data: [] },
      yAxis: { type: 'value' },
      series: [],
    }
  }

  const tooltipFormatter = (params: any) => {
    const val = formatter ? formatter(params[0].value) : (params[0].value?.toFixed(2) ?? '0')
    const unitStr = unit ? ` ${unit}` : ''
    return `${params[0].axisValue}<br/>${val}${unitStr}`
  }

  return {
    title: { text: title, left: 'center', textStyle: { fontSize: 14 } },
    tooltip: {
      trigger: 'axis',
      formatter: tooltipFormatter,
    },
    grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: data.times },
    yAxis: {
      type: 'value',
      name: unit || undefined,
      axisLabel: formatter ? { formatter } : undefined,
    },
    series: [{
      type: 'line',
      smooth: true,
      data: data.values,
      connectNulls: true,
      itemStyle: { color },
      areaStyle: { opacity: 0.1 },
    }],
  }
}

/**
 * 构建通用单 series 柱状图配置
 */
export const buildBarChartOption = (
  title: string,
  categories: string[],
  seriesList: Array<{ name: string; data: number[]; color: string }>,
  yAxisName: string = '',
): Record<string, any> => {
  return {
    title: { text: title, left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { trigger: 'axis' },
    legend: { data: seriesList.map(s => s.name), top: 25 },
    grid: { left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true },
    xAxis: { type: 'category', data: categories },
    yAxis: { type: 'value', name: yAxisName || undefined },
    series: seriesList.map(s => ({
      name: s.name,
      type: 'bar',
      data: s.data,
      itemStyle: { color: s.color },
    })),
  }
}

/**
 * 构建多 series 折线图配置（按 broker 分组）
 * 用于 Broker 监控 Tab
 */
export const buildMultiSeriesChartOption = (
  title: string,
  data: Record<string, { times: string[]; values: number[] }> | { times: string[]; values: number[] } | undefined,
  _color: string,
  yAxisName: string,
  tooltipFormatter?: (value: number) => string,
  fullTimes?: string[],
): Record<string, any> => {
  // 安全处理：data 可能是 undefined 或格式不对
  let safeData: Record<string, { times: string[]; values: number[] }> = {}
  if (data && typeof data === 'object') {
    if ('times' in data && 'values' in data) {
      safeData = { '0': data as { times: string[]; values: number[] } }
    } else {
      safeData = data as Record<string, { times: string[]; values: number[] }>
    }
  }
  const entries = Object.entries(safeData).filter(([, d]) => d && d.times && d.values)
  if (entries.length === 0) {
    return {
      title: { text: title, left: 'center', textStyle: { fontSize: 14, color: '#999' } },
      graphic: { type: 'text', left: 'center', top: 'middle', style: { text: '暂无数据', fill: '#999', fontSize: 14 } },
      xAxis: { type: 'category', data: [] },
      yAxis: { type: 'value' },
      series: [],
    }
  }

  const allTimes = new Set<string>()
  if (fullTimes) {
    fullTimes.forEach(t => allTimes.add(t))
  } else {
    entries.forEach(([, d]) => d.times.forEach(t => allTimes.add(t)))
  }
  const times = Array.from(allTimes).sort()

  return {
    title: { text: title, left: 'center', textStyle: { fontSize: 14 } },
    tooltip: {
      trigger: 'axis',
      formatter: (params: any[]) => {
        if (!params || params.length === 0) return ''
        let html = params[0].axisValue + '<br/>'
        params.forEach((p: any) => {
          const val = tooltipFormatter ? tooltipFormatter(p.value) : (p.value?.toFixed(2) ?? '0')
          html += `${p.marker} Broker ${p.seriesName}: ${val}<br/>`
        })
        return html
      },
    },
    legend: { data: entries.map(([id]) => `Broker ${id}`), top: 25, type: 'scroll' },
    grid: { left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: times },
    yAxis: { type: 'value', name: yAxisName },
    series: entries.map(([id, d], index) => ({
      name: `Broker ${id}`,
      type: 'line',
      smooth: true,
      data: times.map(t => {
        const idx = d.times.indexOf(t)
        return idx >= 0 ? d.values[idx] : null
      }),
      itemStyle: { color: MULTI_SERIES_COLORS[index % MULTI_SERIES_COLORS.length] },
      connectNulls: true,
    })),
  }
}

/**
 * 构建多 series 折线图配置（按分区分组）
 * 用于 Topic 监控 Tab 的分区级图表
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

  if (filteredMetrics.length === 0) {
    return {
      title: { text: title, left: 'center', textStyle: { fontSize: 14, color: '#999' } },
      graphic: { type: 'text', left: 'center', top: 'middle', style: { text: emptyText, fill: '#999', fontSize: 14 } },
      xAxis: { type: 'category', data: [] },
      yAxis: { type: 'value' },
      series: [],
    }
  }

  const times = filteredMetrics[0]?.values.map(v => v.time) || []

  return {
    title: { text: title, left: 'center', textStyle: { fontSize: 14 } },
    tooltip: {
      trigger: 'axis',
      formatter: (params: any[]) => {
        if (!params || params.length === 0) return ''
        let html = params[0].axisValue + '<br/>'
        params.filter(p => p.value !== undefined && p.value !== null).forEach(p => {
          const val = tooltipFormatter ? tooltipFormatter(p.value) : p.value.toFixed(2)
          html += `${p.marker} 分区${p.seriesName}: ${val}<br/>`
        })
        return html
      },
    },
    legend: {
      data: filteredMetrics.map(p => `分区${p.partition}`),
      top: 25,
      type: 'scroll',
    },
    grid: { left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: times },
    yAxis: { type: 'value', name: yAxisName },
    series: filteredMetrics.map((p, index) => ({
      name: p.partition.toString(),
      type: 'line',
      smooth: true,
      data: p.values.map(v => v.value),
      itemStyle: { color: MULTI_SERIES_COLORS[index % MULTI_SERIES_COLORS.length] },
      emphasis: { focus: 'series' },
    })),
  }
}
