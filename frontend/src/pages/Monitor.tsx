import { useState } from 'react'
import { Card, Select, DatePicker, Row, Col, Statistic } from 'antd'
import ReactECharts from 'echarts-for-react'

const { RangePicker } = DatePicker

const Monitor: React.FC = () => {
  const [clusterId, setClusterId] = useState<number | undefined>()
  const [timeRange, setTimeRange] = useState<[string, string] | null>(null)

  const clusterOptions = [
    { value: 1, label: '测试集群' },
  ]

  const getChartOption = (title: string) => ({
    title: { text: title, left: 'center' },
    tooltip: { trigger: 'axis' },
    xAxis: { type: 'category', data: ['00:00', '04:00', '08:00', '12:00', '16:00', '20:00'] },
    yAxis: { type: 'value' },
    series: [{
      data: [120, 200, 150, 80, 70, 110],
      type: 'line',
      smooth: true,
    }],
  })

  return (
    <div>
      <h1 style={{ marginBottom: 24 }}>监控中心</h1>
      
      <Card style={{ marginBottom: 16 }}>
        <Space>
          <Select
            placeholder="选择集群"
            style={{ width: 200 }}
            options={clusterOptions}
            onChange={setClusterId}
            allowClear
          />
          <RangePicker 
            showTime 
            onChange={(dates) => {
              if (dates) {
                setTimeRange([dates[0]?.toISOString() || '', dates[1]?.toISOString() || ''])
              }
            }}
          />
        </Space>
      </Card>

      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card>
            <Statistic title="Broker 数量" value={3} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Topic 数量" value={25} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="消息速率" value={1234} suffix="msg/s" />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="字节速率" value={5.6} suffix="MB/s" />
          </Card>
        </Col>
      </Row>

      <Row gutter={16}>
        <Col span={12}>
          <Card title="消息流入速率">
            <ReactECharts option={getChartOption('')} style={{ height: 300 }} />
          </Card>
        </Col>
        <Col span={12}>
          <Card title="消息流出速率">
            <ReactECharts option={getChartOption('')} style={{ height: 300 }} />
          </Card>
        </Col>
      </Row>
    </div>
  )
}

export default Monitor