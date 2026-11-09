// Topic 批量导入共享工具：申请工单（TicketCreate）与 Topic 管理（ClusterTopics）使用同一 CSV 格式与校验规则

// 批量导入 CSV 固定格式（列顺序固定，首行为表头；UTF-8 BOM 保证 Excel 打开中文不乱码）
export const TOPIC_CSV_TEMPLATE =
  '\ufefftopic_name,partitions,replication_factor,retention_days,business_desc\n' +
  'trade.ods.order-created,6,3,3,订单创建事件流，供风控和实时大盘消费\n' +
  'trade.ods.order-paid,6,3,3,订单支付事件流，供下游实时计算消费\n'

// 单次批量导入最大行数
export const MAX_TOPIC_BATCH_ROWS = 100

// 批量导入行（含校验结果）
export interface TopicBatchRow {
  line: number
  topic_name: string
  partitions: number
  replication_factor: number
  retention_days: number
  business_desc: string
  errors: string[]
}

export const TOPIC_NAME_REGEX = /^[a-zA-Z0-9._-]{1,249}$/

const topicNameRegex = TOPIC_NAME_REGEX

// 解析单行 CSV（支持引号包裹的字段，业务说明含逗号时需用引号包裹）
export function parseCSVLine(line: string): string[] {
  const fields: string[] = []
  let current = ''
  let inQuotes = false
  for (let i = 0; i < line.length; i++) {
    const ch = line[i]
    if (inQuotes) {
      if (ch === '"') {
        if (line[i + 1] === '"') {
          current += '"'
          i++
        } else {
          inQuotes = false
        }
      } else {
        current += ch
      }
    } else if (ch === '"') {
      inQuotes = true
    } else if (ch === ',') {
      fields.push(current)
      current = ''
    } else {
      current += ch
    }
  }
  fields.push(current)
  return fields.map(f => f.trim())
}

// 校验单行（与后端工单/创建接口规则一致）
function validateRow(fields: string[], line: number, seen: Set<string>): TopicBatchRow {
  const row: TopicBatchRow = {
    line,
    topic_name: fields[0] || '',
    partitions: parseInt(fields[1] || '', 10),
    replication_factor: parseInt(fields[2] || '', 10),
    retention_days: fields[3] === '' || fields[3] === undefined ? 3 : parseInt(fields[3], 10),
    business_desc: fields[4] || '',
    errors: [],
  }
  if (!topicNameRegex.test(row.topic_name) || row.topic_name === '.' || row.topic_name === '..') {
    row.errors.push('topic 名称不合法')
  }
  if (seen.has(row.topic_name)) {
    row.errors.push('文件内重复')
  }
  if (!Number.isInteger(row.partitions) || row.partitions < 1 || row.partitions > 200) {
    row.errors.push('分区数需为 1-200')
  }
  if (!Number.isInteger(row.replication_factor) || row.replication_factor < 1 || row.replication_factor > 5) {
    row.errors.push('副本数需为 1-5')
  }
  if (!Number.isInteger(row.retention_days) || row.retention_days < 0 || row.retention_days > 3650) {
    row.errors.push('保留天数需为 0-3650')
  }
  if (!row.business_desc) {
    row.errors.push('业务说明不能为空')
  }
  if (topicNameRegex.test(row.topic_name)) {
    seen.add(row.topic_name)
  }
  return row
}

export interface ParseTopicBatchResult {
  rows: TopicBatchRow[]
  // 整个文件级别错误（表头不符 / 超行数上限），rows 为空
  error?: string
}

// 解析并校验整个 CSV 文本
export function parseTopicBatchCSV(text: string): ParseTopicBatchResult {
  const lines = text.replace(/^\ufeff/, '').split(/\r?\n/).filter(l => l.trim() !== '')
  if (lines.length === 0) {
    return { rows: [], error: '文件为空' }
  }
  const header = parseCSVLine(lines[0]).map(h => h.toLowerCase())
  if (header.join(',') !== 'topic_name,partitions,replication_factor,retention_days,business_desc') {
    return { rows: [], error: '表头不符合模板格式，请下载模板后按列填写' }
  }
  if (lines.length - 1 > MAX_TOPIC_BATCH_ROWS) {
    return { rows: [], error: `单次最多导入 ${MAX_TOPIC_BATCH_ROWS} 行` }
  }
  const seen = new Set<string>()
  const rows: TopicBatchRow[] = []
  for (let i = 1; i < lines.length; i++) {
    const fields = parseCSVLine(lines[i])
    if (fields.length !== 5) {
      rows.push({
        line: i + 1,
        topic_name: fields[0] || '',
        partitions: NaN,
        replication_factor: NaN,
        retention_days: NaN,
        business_desc: '',
        errors: [`列数错误（${fields.length} 列，应为 5 列）`],
      })
      continue
    }
    rows.push(validateRow(fields, i + 1, seen))
  }
  return { rows }
}

// 下载 CSV 模板（UTF-8 BOM）
export function downloadTopicTemplate() {
  const blob = new Blob([TOPIC_CSV_TEMPLATE], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'topic_batch_template.csv'
  a.click()
  URL.revokeObjectURL(url)
}
