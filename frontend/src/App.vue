<template>
  <div id="app">
    <a-layout class="layout">
      <a-layout-header class="header">
        <div class="logo">
          <icon-code-square :size="28" />
          <span>Pprof Trend Analyzer</span>
        </div>
      </a-layout-header>
      <a-layout-content class="content">
        <!-- Control Panel -->
        <a-card class="control-card" :body-style="{ padding: '16px' }">
          <a-tabs default-active-key="1" size="small">
            <a-tab-pane key="1" title="📁 目录分析">
              <a-space direction="vertical" :size="10" style="width: 100%">
                <a-alert type="info" :closable="false" style="margin-bottom: 8px;">
                  分析已有的 pprof 文件目录，查看历史趋势。每次分析会清空之前的数据。
                </a-alert>
                <a-input
                  v-model="directory"
                  placeholder="例如: /Users/lixiang/workspace/private/code/pprof/test_pprof_files"
                  size="default"
                >
                  <template #prepend>
                    <span style="margin-right: 4px;">Pprof 文件目录</span>
                    <icon-folder />
                  </template>
                </a-input>
                <a-button
                  type="primary"
                  size="default"
                  long
                  @click="analyzeDirectory"
                  :loading="loading"
                >
                  <template #icon>
                    <icon-search />
                  </template>
                  分析趋势
                </a-button>
              </a-space>
            </a-tab-pane>
            
            <a-tab-pane key="2" title="🔄 实时采集">
              <a-space direction="vertical" :size="10" style="width: 100%">
                <a-alert type="info" :closable="false" style="margin-bottom: 8px;">
                  从运行中的服务实时采集 pprof 数据，自动分析并更新趋势图表。每次启动会创建新的时间戳目录。
                </a-alert>
                
                <a-row :gutter="12">
                  <a-col :span="8">
                    <a-input
                      v-model="collectorConfig.baseURL"
                      placeholder="http://localhost:6060"
                      size="default"
                    >
                      <template #prepend>
                        <span style="margin-right: 4px;">目标服务 URL</span>
                        <icon-link />
                      </template>
                    </a-input>
                  </a-col>
                  <a-col :span="4">
                    <a-input-number
                      v-model="collectorConfig.interval"
                      :min="1"
                      :max="3600"
                      size="default"
                      style="width: 100%"
                    >
                      <template #prepend>
                        <span style="margin-right: 4px;">采集间隔</span>
                        <icon-clock-circle />
                      </template>
                      <template #suffix>秒</template>
                    </a-input-number>
                  </a-col>
                  <a-col :span="4">
                    <a-input-number
                      v-model="refreshInterval"
                      :min="1"
                      :max="60"
                      size="default"
                      style="width: 100%"
                    >
                      <template #prepend>
                        <span style="margin-right: 4px;">刷新间隔</span>
                        <icon-refresh />
                      </template>
                      <template #suffix>秒</template>
                    </a-input-number>
                  </a-col>
                  <a-col :span="8">
                    <a-input
                      v-model="collectorConfig.outputDir"
                      placeholder="/tmp/pprof_collected"
                      size="default"
                    >
                      <template #prepend>
                        <span style="margin-right: 4px;">输出基础目录</span>
                        <icon-folder />
                      </template>
                    </a-input>
                  </a-col>
                </a-row>

                <a-select
                  v-model="collectorConfig.profileTypes"
                  placeholder="选择采集类型: heap, profile, goroutine, allocs, block, mutex"
                  multiple
                  size="default"
                  style="width: 100%"
                >
                  <template #prefix>
                    <span style="margin-right: 4px;">采集类型</span>
                  </template>
                  <a-option value="heap">heap</a-option>
                  <a-option value="profile">profile</a-option>
                  <a-option value="goroutine">goroutine</a-option>
                  <a-option value="allocs">allocs</a-option>
                  <a-option value="block">block</a-option>
                  <a-option value="mutex">mutex</a-option>
                </a-select>

                <a-space style="width: 100%; justify-content: space-between;">
                  <a-space>
                    <a-button
                      type="primary"
                      size="default"
                      @click="startCollector"
                      :loading="collectorLoading"
                      :disabled="collectorStatus.running"
                    >
                      <template #icon>
                        <icon-play-arrow />
                      </template>
                      开始采集
                    </a-button>
                    <a-button
                      status="danger"
                      size="default"
                      @click="stopCollector"
                      :disabled="!collectorStatus.running"
                    >
                      <template #icon>
                        <icon-pause />
                      </template>
                      停止采集
                    </a-button>
                    <a-tag v-if="collectorStatus.running" color="green">
                      <icon-check-circle /> 采集中
                    </a-tag>
                  </a-space>
                  <span v-if="currentCollectionDir" style="font-size: 12px; color: #86909c;">
                    <icon-folder :size="14" /> 当前采集目录: <strong>{{ currentCollectionDir }}</strong>
                  </span>
                </a-space>
              </a-space>
            </a-tab-pane>
          </a-tabs>
        </a-card>

        <div v-if="trends && Object.keys(trends).length > 0">
          <!-- Profile Type Selector -->
          <a-card class="selector-card" :body-style="{ padding: '12px' }">
            <a-space wrap :size="8">
              <span style="font-size: 13px; color: #4e5969; margin-right: 8px;">选择 Profile 类型:</span>
              <a-tag
                v-for="(trend, type) in trends"
                :key="type"
                :color="selectedType === type ? 'arcoblue' : 'gray'"
                :checkable="true"
                :checked="selectedType === type"
                @click="selectType(type)"
                class="type-tag"
              >
                {{ getProfileTypeTitle(type) }}
                <span class="sample-count">({{ trend.timestamps.length }} 样本)</span>
              </a-tag>
            </a-space>
          </a-card>

          <!-- Charts -->
          <div v-if="selectedType && trends[selectedType]">
            <!-- Function Level Trend -->
            <a-card class="chart-card" :body-style="{ padding: '16px' }">
              <template #title>
                <div style="display: flex; justify-content: space-between; align-items: center;">
                  <span style="font-size: 14px;">📊 函数级别趋势</span>
                  <a-space :size="8">
                    <span style="font-size: 12px;">显示 Top</span>
                    <a-input-number 
                      v-model="topNConfig[selectedType]" 
                      :min="5" 
                      :max="50" 
                      :default-value="10"
                      size="mini"
                      style="width: 80px;"
                      @change="updateCharts"
                    />
                    <span style="font-size: 12px;">个函数</span>
                  </a-space>
                </div>
              </template>
              <template #extra>
                <a-space :size="4">
                  <a-tag color="blue" size="small">点击图例：单选</a-tag>
                  <a-tag color="purple" size="small">Shift+点击：多选</a-tag>
                  <a-tag color="orange" size="small">再次点击：显示全部</a-tag>
                </a-space>
              </template>
              <div ref="functionChartRef" class="chart-medium"></div>
            </a-card>

            <!-- Overall Trend -->
            <a-card class="chart-card" :body-style="{ padding: '16px' }">
              <template #title>
                <span style="font-size: 14px;">📈 总体趋势</span>
              </template>
              <div ref="overallChartRef" class="chart-small"></div>
            </a-card>
          </div>
        </div>

        <a-empty v-else-if="!loading" description="请选择分析模式：目录分析或实时采集" style="margin-top: 40px;" />
      </a-layout-content>
    </a-layout>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { Message } from '@arco-design/web-vue'
import {
  IconCodeSquare,
  IconFolder,
  IconSearch,
  IconLink,
  IconClockCircle,
  IconPlayArrow,
  IconPause,
  IconCheckCircle,
  IconRefresh
} from '@arco-design/web-vue/es/icon'
import * as echarts from 'echarts'
import axios from 'axios'

const directory = ref('/Users/lixiang/workspace/private/code/pprof/test_pprof_files')
const loading = ref(false)
const trends = ref(null)
const selectedType = ref(null)
const topNConfig = ref({})
const functionChartRef = ref(null)
const overallChartRef = ref(null)
const currentCollectionDir = ref('')
const refreshInterval = ref(5)

// Collector config
const collectorConfig = ref({
  baseURL: 'http://localhost:6060',
  interval: 5,
  outputDir: '/tmp/pprof_collected',
  profileTypes: ['heap']
})
const collectorLoading = ref(false)
const collectorStatus = ref({ running: false })
let pollingInterval = null

let functionChart = null
let overallChart = null

const getProfileTypeTitle = (type) => {
  // 不添加中文翻译，直接返回类型名称
  return type
}

const selectType = (type) => {
  selectedType.value = type
  nextTick(() => {
    updateCharts()
  })
}

const analyzeDirectory = async () => {
  if (!directory.value) {
    Message.warning('请输入目录路径')
    return
  }

  loading.value = true
  currentCollectionDir.value = ''
  try {
    await axios.post('/api/analyze', { directory: directory.value })
    await fetchTrends()
    Message.success('分析完成！')
  } catch (error) {
    Message.error('分析失败: ' + (error.response?.data?.error || error.message))
  } finally {
    loading.value = false
  }
}

const fetchTrends = async () => {
  const response = await axios.get('/api/trends')
  trends.value = response.data

  // Initialize topNConfig and select first type
  Object.keys(trends.value).forEach(type => {
    if (!topNConfig.value[type]) {
      topNConfig.value[type] = 10
    }
  })

  // Select first type by default if not selected
  if (!selectedType.value) {
    const types = Object.keys(trends.value)
    if (types.length > 0) {
      selectedType.value = types[0]
    }
  }

  await nextTick()
  updateCharts()
}

const startCollector = async () => {
  if (!collectorConfig.value.baseURL || !collectorConfig.value.outputDir) {
    Message.warning('请填写完整的采集配置')
    return
  }

  if (collectorConfig.value.profileTypes.length === 0) {
    Message.warning('请至少选择一种采集类型')
    return
  }

  collectorLoading.value = true
  try {
    const response = await axios.post('/api/collector/start', collectorConfig.value)
    currentCollectionDir.value = response.data.outputDir
    Message.success('实时采集已启动！')
    checkCollectorStatus()
    startPolling()
  } catch (error) {
    Message.error('启动失败: ' + (error.response?.data?.error || error.message))
  } finally {
    collectorLoading.value = false
  }
}

const stopCollector = async () => {
  try {
    await axios.post('/api/collector/stop')
    Message.success('实时采集已停止')
    checkCollectorStatus()
    stopPolling()
  } catch (error) {
    Message.error('停止失败: ' + (error.response?.data?.error || error.message))
  }
}

const checkCollectorStatus = async () => {
  try {
    const response = await axios.get('/api/collector/status')
    collectorStatus.value = response.data
    if (response.data.outputDir) {
      currentCollectionDir.value = response.data.outputDir
    }
  } catch (error) {
    console.error('Failed to check collector status:', error)
  }
}

const startPolling = () => {
  stopPolling()
  const intervalMs = (refreshInterval.value || 5) * 1000
  pollingInterval = setInterval(async () => {
    try {
      await fetchTrends()
    } catch (error) {
      console.error('Failed to fetch trends:', error)
    }
  }, intervalMs)
}

const stopPolling = () => {
  if (pollingInterval) {
    clearInterval(pollingInterval)
    pollingInterval = null
  }
}

const updateCharts = () => {
  if (!selectedType.value || !trends.value || !trends.value[selectedType.value]) {
    return
  }

  const trend = trends.value[selectedType.value]
  renderFunctionsChart(trend)
  renderOverallChart(trend)
}

const renderFunctionsChart = (trend) => {
  if (!functionChartRef.value) return

  const topN = topNConfig.value[selectedType.value] || 10
  const topFunctions = getTopFunctions(trend, topN)
  
  if (!functionChart) {
    functionChart = echarts.init(functionChartRef.value)
  }

  const series = topFunctions.map(func => ({
    name: func.name.split('/').pop().substring(0, 80),
    type: 'line',
    data: func.values,
    smooth: true,
    symbol: 'circle',
    symbolSize: 4
  }))

  const option = {
    tooltip: {
      trigger: 'axis',
      formatter: (params) => {
        let result = `<strong>${params[0].name}</strong><br/>`
        params.forEach(param => {
          result += `${param.marker}${param.seriesName}: ${formatValue(param.value, trend.overall.unit)}<br/>`
        })
        return result
      }
    },
    legend: {
      type: 'scroll',
      orient: 'vertical',
      right: 10,
      top: 'middle',
      data: series.map(s => s.name),
      textStyle: {
        fontSize: 11
      },
      width: 280,
      selectedMode: 'multiple'
    },
    xAxis: {
      type: 'category',
      data: trend.timestamps,
      axisLabel: {
        rotate: 0,
        interval: 'auto',
        fontSize: 10,
        showMaxLabel: true,
        showMinLabel: true
      }
    },
    yAxis: {
      type: 'value',
      name: trend.overall.unit,
      axisLabel: {
        formatter: (value) => formatValue(value, trend.overall.unit),
        fontSize: 10
      },
      nameTextStyle: {
        fontSize: 11
      }
    },
    series: series,
    grid: {
      left: 70,
      right: 300,
      top: 30,
      bottom: 40
    }
  }
  
  // 修复后的图例交互逻辑
  // 默认显示所有函数
  // 点击某个函数 -> 只显示该函数
  // 按住 Shift 点击 -> 多选函数
  // 再次点击已选中的函数 -> 显示所有函数
  functionChart.off('legendselectchanged')
  functionChart.on('legendselectchanged', function(params) {
    const selected = params.selected
    const clickedName = params.name
    const allNames = Object.keys(selected)
    const selectedCount = allNames.filter(name => selected[name]).length
    const wasSelected = !selected[clickedName] // 点击前的状态
    
    // 如果点击前该项是选中的，且当前只有一个或多个选中，再次点击应该显示全部
    if (wasSelected && selectedCount === 0) {
      // 用户取消了最后一个选中项，显示全部
      const newSelected = {}
      allNames.forEach(name => {
        newSelected[name] = true
      })
      functionChart.setOption({ legend: { selected: newSelected } })
    } else if (selectedCount === allNames.length - 1 && !wasSelected) {
      // 从全部显示状态点击了一个，只显示这一个
      const newSelected = {}
      allNames.forEach(name => {
        newSelected[name] = name === clickedName
      })
      functionChart.setOption({ legend: { selected: newSelected } })
    }
    // 其他情况使用默认行为（支持 Shift 多选）
  })
  
  functionChart.setOption(option, true)
}

const renderOverallChart = (trend) => {
  if (!overallChartRef.value) return

  if (!overallChart) {
    overallChart = echarts.init(overallChartRef.value)
  }

  const option = {
    tooltip: {
      trigger: 'axis',
      formatter: (params) => {
        const param = params[0]
        return `<strong>${param.name}</strong><br/>${param.seriesName}: ${formatValue(param.value, trend.overall.unit)}`
      }
    },
    xAxis: {
      type: 'category',
      data: trend.timestamps,
      axisLabel: {
        rotate: 0,
        interval: 'auto',
        fontSize: 10,
        showMaxLabel: true,
        showMinLabel: true
      }
    },
    yAxis: {
      type: 'value',
      name: trend.overall.unit,
      axisLabel: {
        formatter: (value) => formatValue(value, trend.overall.unit),
        fontSize: 10
      },
      nameTextStyle: {
        fontSize: 11
      }
    },
    series: [
      {
        name: getProfileTypeTitle(selectedType.value),
        type: 'line',
        data: trend.overall.totalValues,
        smooth: true,
        lineStyle: {
          width: 2
        },
        areaStyle: {
          opacity: 0.3
        },
        symbol: 'circle',
        symbolSize: 6
      }
    ],
    grid: {
      left: 70,
      right: 30,
      top: 30,
      bottom: 40
    }
  }
  overallChart.setOption(option, true)
}

const getTopFunctions = (trend, topN = 10) => {
  const functions = Object.values(trend.functions)
  return functions
    .map(func => {
      const sum = func.values.reduce((a, b) => a + b, 0)
      const avg = sum / func.values.length
      return {
        ...func,
        avgValue: avg
      }
    })
    .filter(func => func.avgValue > 0)
    .sort((a, b) => b.avgValue - a.avgValue)
    .slice(0, topN)
}

const formatValue = (value, unit) => {
  if (unit === 'bytes') {
    if (value >= 1024 * 1024 * 1024) {
      return (value / (1024 * 1024 * 1024)).toFixed(2) + ' GB'
    } else if (value >= 1024 * 1024) {
      return (value / (1024 * 1024)).toFixed(2) + ' MB'
    } else if (value >= 1024) {
      return (value / 1024).toFixed(2) + ' KB'
    }
    return value + ' B'
  } else if (unit === 'nanoseconds') {
    if (value >= 1000000000) {
      return (value / 1000000000).toFixed(2) + ' s'
    } else if (value >= 1000000) {
      return (value / 1000000).toFixed(2) + ' ms'
    } else if (value >= 1000) {
      return (value / 1000).toFixed(2) + ' μs'
    }
    return value + ' ns'
  }
  return value.toLocaleString()
}

onMounted(() => {
  window.addEventListener('resize', () => {
    if (functionChart) functionChart.resize()
    if (overallChart) overallChart.resize()
  })
  
  checkCollectorStatus()
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.layout {
  min-height: 100vh;
  background: #f0f2f5;
}

.header {
  background: #fff;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.08);
  padding: 0 20px;
  display: flex;
  align-items: center;
  height: 56px;
}

.logo {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 18px;
  font-weight: bold;
  color: #165dff;
}

.content {
  padding: 16px;
  max-width: 1600px;
  margin: 0 auto;
  width: 100%;
}

.control-card {
  margin-bottom: 12px;
}

.selector-card {
  margin-bottom: 12px;
  background: #fff;
}

.type-tag {
  cursor: pointer;
  font-size: 13px;
  padding: 6px 12px;
  transition: all 0.2s;
}

.type-tag:hover {
  transform: translateY(-1px);
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.1);
}

.sample-count {
  font-size: 11px;
  opacity: 0.8;
  margin-left: 4px;
}

.chart-card {
  margin-bottom: 12px;
  background: #fff;
}

.chart-small {
  width: 100%;
  height: 280px;
}

.chart-medium {
  width: 100%;
  height: 380px;
}
</style>
