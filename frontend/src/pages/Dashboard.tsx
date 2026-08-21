import { useEffect, useState } from 'react'
import EChartsReact from 'echarts-for-react/lib/core'
import echarts from '../utils/echarts'
import { useNavigate } from 'react-router-dom'
import { Spin, Modal } from 'antd'
import { SectionTitle, HealthDot } from '../components/bento'
import { dashboardService, DashboardOverview } from '../services/dashboardService'
import { createDonutChartOption, createHorizontalStackedBarChartOption, DonutDataItem, StackedBarSeries } from '../utils/chartOptions'

const Dashboard: React.FC = () => {
  const [loading, setLoading] = useState(true)
  const [overview, setOverview] = useState<DashboardOverview | null>(null)
  const [lagModalOpen, setLagModalOpen] = useState(false)
  const navigate = useNavigate()

  useEffect(() => {
    loadDashboard()
  }, [])

  const loadDashboard = async () => {
    setLoading(true)
    try {
      const data = await dashboardService.getOverview()
      setOverview(data)
    } catch {
      // Fallback: try the old composite approach
      try {
        const { clusterAPI } = await import('../services/cluster')
        const { metricsAPI } = await import('../services/metrics')
        const api = (await import('../services/api')).default
        const clusterRes = await clusterAPI.list(1, 100)
        const clusters = clusterRes.data || []
        const authMap: Record<string, number> = {}
        clusters.forEach((c: any) => {
          const type = c.auth_type === 'none' ? 'NONE' :
            c.auth_type === 'plaintext' ? 'PLAIN' :
            c.auth_type === 'scram' ? 'SCRAM' :
            c.auth_type === 'kerberos' ? 'Kerberos' : c.auth_type.toUpperCase()
          authMap[type] = (authMap[type] || 0) + 1
        })
        const metricsResults = await Promise.all(
          clusters.map(async (cluster: any) => {
            try {
              const res = await metricsAPI.getClusterMetrics(cluster.cluster_id)
              return { cluster_id: cluster.cluster_id, cluster_name: cluster.cluster_name, data: res.data }
            } catch {
              return { cluster_id: cluster.cluster_id, cluster_name: cluster.cluster_name, data: null }
            }
          })
        )
        let totalTopics = 0
        let totalBrokers = 0
        let totalLag = 0
        let totalCG = 0
        const clusterSizes: DashboardOverview['cluster_sizes'] = []
        let healthy = 0, warning = 0, error = 0, unknown = 0
        metricsResults.forEach(r => {
          if (r.data) {
            totalTopics += r.data.topic_count || 0
            totalBrokers += r.data.broker_count || 0
            totalLag += r.data.consumer_groups?.reduce((s: number, g: any) => s + g.total_lag, 0) || 0
            totalCG += r.data.consumer_groups?.length || 0
            const urp = r.data.broker_metrics?.under_replicated_partitions || 0
            const offline = r.data.broker_metrics?.offline_partitions_count || 0
            // 有 JMX 数据时按 URP/offline 判定；无 JMX 但 AdminClient 可用时默认 healthy
            if (offline > 0) error++
            else if (urp > 0) warning++
            else healthy++
            clusterSizes.push({
              cluster_id: r.cluster_id,
              cluster_name: r.cluster_name,
              broker_count: r.data.broker_count || null,
              topic_count: r.data.topic_count || 0,
              health_status: offline > 0 ? 'error' : urp > 0 ? 'warning' : 'healthy',
            })
          } else {
            unknown++
            clusterSizes.push({
              cluster_id: r.cluster_id,
              cluster_name: r.cluster_name,
              broker_count: null,
              topic_count: 0,
              health_status: 'unknown',
            })
          }
        })
        let totalUsers = 0
        try {
          const userRes = await api.get('/users', { params: { page: 1, page_size: 1 } })
          totalUsers = userRes.data?.total || 0
        } catch { /* non-admin */ }
        setOverview({
          clusters: { total: clusters.length, healthy, warning, error, unknown },
          topics_total: totalTopics,
          brokers_online: totalBrokers > 0 ? totalBrokers : null,
          partitions_total: null,
          users_total: totalUsers,
          consumer_groups: { total: totalCG, total_lag: totalLag },
          consumer_group_details: [],
          auth_type_distribution: authMap,
          cluster_sizes: clusterSizes,
        })
      } catch {
        /* ignore */
      }
    } finally {
      setLoading(false)
    }
  }

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: 400 }}>
        <Spin size="large" />
      </div>
    )
  }

  if (!overview) {
    return <div style={{ textAlign: 'center', padding: 48, color: 'var(--text-3)' }}>无法加载仪表盘数据</div>
  }

  const { clusters, topics_total, brokers_online, partitions_total, users_total, consumer_groups, auth_type_distribution, cluster_sizes } = overview

  // Health ring data
  const healthDonutData: DonutDataItem[] = [
    { name: 'Healthy', value: clusters.healthy, color: '#10b981' },
    { name: 'Warning', value: clusters.warning, color: '#f59e0b' },
    { name: 'Error', value: clusters.error, color: '#ef4444' },
    { name: 'Unknown', value: clusters.unknown, color: '#a8a29e' },
  ].filter(d => d.value > 0)

  // Auth type distribution
  const authDonutData: DonutDataItem[] = Object.entries(auth_type_distribution).map(([name, value]) => {
    const colorMap: Record<string, string> = {
      'none': '#3b82f6',
      'NONE': '#3b82f6',
      'plaintext': '#8b5cf6',
      'PLAIN': '#8b5cf6',
      'scram': '#10b981',
      'SCRAM': '#10b981',
      'kerberos': '#f59e0b',
      'KERBEROS': '#f59e0b',
    }
    // Display name
    const displayMap: Record<string, string> = {
      'none': 'NONE',
      'NONE': 'NONE',
      'plaintext': 'PLAIN',
      'PLAIN': 'PLAIN',
      'scram': 'SCRAM',
      'SCRAM': 'SCRAM',
      'kerberos': 'Kerberos',
      'KERBEROS': 'Kerberos',
    }
    return { name: displayMap[name] || name, value, color: colorMap[name] || '#f97316' }
  })

  // Cluster size stacked bar
  const barCategories = cluster_sizes.map(c => c.cluster_name?.length > 16 ? c.cluster_name.slice(0, 14) + '…' : c.cluster_name || '')
  const barBrokers: StackedBarSeries = {
    name: 'Brokers',
    data: cluster_sizes.map(c => c.broker_count || 0),
    color: '#f97316',
  }
  const barTopics: StackedBarSeries = {
    name: 'Topics',
    data: cluster_sizes.map(c => c.topic_count || 0),
    color: '#0d9488',
  }

  const formatLag = (lag: number): string => {
    if (lag >= 1000000) return (lag / 1000000).toFixed(1) + 'M'
    if (lag >= 1000) return (lag / 1000).toFixed(1) + 'K'
    return lag.toString()
  }

  return (
    <div>
      <div className="page-header">
        <h1>仪表盘</h1>
        <div className="page-accent-line" />
      </div>

      <div className="bento-grid">
        {/* Hero: cluster total + health ring */}
        <div className="bento-card bento-card-dark" style={{ gridColumn: 'span 5', gridRow: 'span 2' }}>
          <div className="bento-card-inner" style={{ background: 'linear-gradient(135deg, #0c0a09 0%, #1c1917 100%)', color: '#fff', justifyContent: 'space-between', padding: 28 }}>
            <div style={{ textAlign: 'center' }}>
              <div className="stat-label" style={{ color: 'rgba(255,255,255,0.45)', letterSpacing: '1.5px' }}>CLUSTERS</div>
            </div>
            {healthDonutData.length > 0 && (
              <EChartsReact echarts={echarts}
                option={{
                  ...createDonutChartOption(healthDonutData, ['50%', '75%']),
                  graphic: [
                    {
                      type: 'text',
                      left: 'center',
                      top: '36%',
                      style: {
                        text: String(clusters.total),
                        fontSize: 42,
                        fontWeight: 800,
                        fontFamily: "'JetBrains Mono', monospace",
                        fill: '#fff',
                        textAlign: 'center',
                      },
                    },
                    {
                      type: 'text',
                      left: 'center',
                      top: '60%',
                      style: {
                        text: 'CLUSTERS',
                        fontSize: 11,
                        fontWeight: 600,
                        fill: 'rgba(255,255,255,0.4)',
                        textAlign: 'center',
                        letterSpacing: '1.5px',
                      },
                    },
                  ],
                }}
                style={{ width: '100%', height: 200 }}
                notMerge={true}
              />
            )}
            <div style={{ display: 'flex', gap: 16, justifyContent: 'center' }}>
              {clusters.healthy > 0 && (
                <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 12, fontWeight: 600, color: '#10b981' }}>
                  <span style={{ width: 8, height: 8, borderRadius: '50%', background: '#10b981', display: 'inline-block' }} />
                  {clusters.healthy}
                </span>
              )}
              {clusters.warning > 0 && (
                <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 12, fontWeight: 600, color: '#f59e0b' }}>
                  <span style={{ width: 8, height: 8, borderRadius: '50%', background: '#f59e0b', display: 'inline-block' }} />
                  {clusters.warning}
                </span>
              )}
              {clusters.error > 0 && (
                <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 12, fontWeight: 600, color: '#ef4444' }}>
                  <span style={{ width: 8, height: 8, borderRadius: '50%', background: '#ef4444', display: 'inline-block' }} />
                  {clusters.error}
                </span>
              )}
              {clusters.unknown > 0 && (
                <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 12, fontWeight: 600, color: '#a8a29e' }}>
                  <span style={{ width: 8, height: 8, borderRadius: '50%', background: '#a8a29e', display: 'inline-block' }} />
                  {clusters.unknown}
                </span>
              )}
            </div>
          </div>
        </div>

        {/* Row 1: TOPIC TOTAL (3) + BROKERS ONLINE (4) */}
        <div className="bento-card" style={{ gridColumn: 'span 3' }}>
          <div className="bento-card-inner">
            <div className="stat-label">TOPIC TOTAL</div>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginTop: 8 }}>
              <span className="stat-value">{topics_total.toLocaleString()}</span>
            </div>
            <span className="stat-delta up">&#9650; {topics_total} total</span>
          </div>
        </div>
        <div className="bento-card" style={{ gridColumn: 'span 4' }}>
          <div className="bento-card-inner">
            <div className="stat-label">BROKERS ONLINE</div>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginTop: 8 }}>
              <span className="stat-value" style={brokers_online && brokers_online > 0 ? { color: '#10b981' } : undefined}>{brokers_online ?? '—'}</span>
              {brokers_online != null && brokers_online > 0 && <span style={{ fontSize: 16, color: 'var(--text-3)', fontWeight: 500 }}>/ {brokers_online}</span>}
            </div>
            {brokers_online === 0 && <span className="stat-delta down">&#9660; 0 online</span>}
          </div>
        </div>

        {/* Row 2: PARTITIONS (4) + USERS (3) */}
        <div className="bento-card" style={{ gridColumn: 'span 4' }}>
          <div className="bento-card-inner">
            <div className="stat-label">PARTITIONS</div>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginTop: 8 }}>
              <span className="stat-value">{partitions_total != null ? partitions_total.toLocaleString() : '—'}</span>
            </div>
            <span className="stat-delta neutral">stable</span>
          </div>
        </div>
        <div className="bento-card" style={{ gridColumn: 'span 3' }}>
          <div className="bento-card-inner" style={{ cursor: 'pointer' }} onClick={() => navigate('/users')}>
            <div className="stat-label">USERS</div>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginTop: 8 }}>
              <span className="stat-value">{users_total}</span>
            </div>
          </div>
        </div>

        {/* Row 3: CONSUMER GROUPS (4) + CONSUMER GROUP LAG (4) */}
        <div className="bento-card" style={{ gridColumn: 'span 4' }}>
          <div className="bento-card-inner">
            <div className="stat-label">CONSUMER GROUPS</div>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginTop: 8 }}>
              <span className="stat-value">{consumer_groups?.total?.toLocaleString() ?? 0}</span>
            </div>
          </div>
        </div>
        <div className="bento-card" style={{ gridColumn: 'span 4', cursor: 'pointer' }} onClick={() => setLagModalOpen(true)}>
          <div className="bento-card-inner">
            <div className="stat-label">CONSUMER GROUP LAG</div>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginTop: 8 }}>
              <span className="stat-value" style={consumer_groups && consumer_groups.total_lag > 0 ? { color: '#ef4444' } : undefined}>{consumer_groups ? formatLag(consumer_groups.total_lag) : '0'}</span>
            </div>
            {consumer_groups && consumer_groups.total_lag > 100000 && (
              <span className="stat-delta down">&#9660; high lag</span>
            )}
          </div>
        </div>

        {/* Auth type distribution */}
        <div className="bento-card" style={{ gridColumn: 'span 4' }}>
          <div className="bento-card-inner">
            <SectionTitle title="认证类型" />
            {authDonutData.length > 0 && (
              <EChartsReact echarts={echarts}
                option={createDonutChartOption(authDonutData)}
                style={{ width: '100%', height: 180 }}
                notMerge={true}
              />
            )}
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px 16px', marginTop: 'auto' }}>
              {authDonutData.map(d => (
                <span key={d.name} style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, fontWeight: 600, color: 'var(--text-2)' }}>
                  <span style={{ width: 7, height: 7, borderRadius: 2, background: d.color, display: 'inline-block' }} />
                  {d.name} {d.value}
                </span>
              ))}
            </div>
          </div>
        </div>

        {/* Cluster size stacked bar */}
        <div className="bento-card" style={{ gridColumn: 'span 12' }}>
          <div className="bento-card-inner">
            <SectionTitle title="集群规模概览" />
            <EChartsReact echarts={echarts}
              option={createHorizontalStackedBarChartOption(barCategories, [barBrokers, barTopics], '数量')}
              style={{ width: '100%', height: 220 }}
              notMerge={true}
            />
          </div>
        </div>

        {/* Cluster health list - 占满整行，分成两列 */}
        <div className="bento-card" style={{ gridColumn: 'span 12' }}>
          <div className="bento-card-inner">
            <SectionTitle title="集群健康状态" />
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '2px 24px' }}>
              {cluster_sizes.map(c => {
                const statusMap: Record<string, { label: string; color: string }> = {
                  healthy: { label: '正常', color: '#10b981' },
                  warning: { label: '副本不足', color: '#f59e0b' },
                  error: { label: '有离线分区', color: '#ef4444' },
                  unknown: { label: '未知', color: '#a8a29e' },
                }
                const s = statusMap[c.health_status] || statusMap.unknown
                return (
                  <div key={c.cluster_id} className="cluster-card-row" onClick={() => navigate(`/clusters/monitor?clusterId=${c.cluster_id}`)}>
                    <HealthDot status={c.health_status as any} />
                    <span style={{ fontWeight: 600, flex: 1 }}>{c.cluster_name}</span>
                    {c.broker_count !== null && (
                      <span className="mono" style={{ fontSize: 13, color: 'var(--text-3)' }}>{c.broker_count} brokers</span>
                    )}
                    <span className="mono" style={{ fontSize: 13, color: c.health_status === 'healthy' ? 'var(--text-3)' : s.color, marginLeft: 16 }}>
                      {c.topic_count} topics
                    </span>
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      </div>
      <Modal
        title="消费组 Lag 详情"
        open={lagModalOpen}
        onCancel={() => setLagModalOpen(false)}
        footer={null}
        width={800}
      >
        <div className="bento-table-header" style={{ gridTemplateColumns: '100px minmax(180px, 1.5fr) minmax(120px, 1fr) 90px 70px' }}>
          <div>集群</div>
          <div>Consumer Group</div>
          <div>Topic</div>
          <div style={{ textAlign: 'right' }}>Lag</div>
          <div style={{ textAlign: 'right' }}>Members</div>
        </div>
        <div className="bento-table-body">
          {(!overview?.consumer_group_details || overview.consumer_group_details.length === 0) ? (
            <div style={{ textAlign: 'center', padding: 24, color: 'var(--text-3)' }}>暂无 Lag 数据</div>
          ) : (
            overview.consumer_group_details
              .filter(item => item.total_lag > 0)
              .sort((a, b) => b.total_lag - a.total_lag)
              .map((item, i) => (
                <div key={i} className="bento-table-row" style={{ gridTemplateColumns: '100px minmax(180px, 1.5fr) minmax(120px, 1fr) 90px 70px', cursor: 'pointer' }}
                  onClick={() => { setLagModalOpen(false); navigate(`/clusters/monitor?clusterId=${item.cluster_id}&tab=topic&topicName=${encodeURIComponent(item.topic || '')}&consumerGroup=${encodeURIComponent(item.group_id)}`) }}>
                  <span style={{ fontSize: 12 }} title={item.cluster_name}>{item.cluster_name}</span>
                  <span className="bento-table-cell-wrap" style={{ fontSize: 12, fontFamily: 'var(--font-mono)', wordBreak: 'break-all', color: 'var(--brand)', fontWeight: 600 }} title={item.group_id}>{item.group_id}</span>
                  <span className="bento-table-cell-wrap" style={{ fontSize: 12 }} title={item.topic}>{item.topic || '-'}</span>
                  <span style={{ fontSize: 12, textAlign: 'right', color: item.total_lag > 0 ? '#ef4444' : 'var(--text-3)', fontFamily: 'var(--font-mono)' }}>{item.total_lag?.toLocaleString() ?? 0}</span>
                  <span style={{ fontSize: 12, textAlign: 'right' }}>{item.member_count ?? '-'}</span>
                </div>
              ))
          )}
        </div>
      </Modal>
    </div>
  )
}

export default Dashboard
