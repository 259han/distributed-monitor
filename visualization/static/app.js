class MonitorApp {
    constructor() {
        this.ws = null;
        this.charts = {};
        this.isConnected = false;
        this.currentMetrics = {}; // 添加当前指标数据存储
        
        this.init();
    }

    init() {
        this.initCharts();
        this.connectWebSocket();
    }

    connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

        console.log('Connecting to WebSocket:', wsUrl);
        this.ws = new WebSocket(wsUrl);
        
        this.ws.onopen = () => {
            this.isConnected = true;
            this.updateConnectionStatus();
            console.log('WebSocket connected');
        };

        this.ws.onmessage = (event) => {
            this.handleWebSocketMessage(event.data);
        };

        this.ws.onclose = () => {
            this.isConnected = false;
            this.updateConnectionStatus();
            console.log('WebSocket disconnected');
            // 尝试重连
            setTimeout(() => this.connectWebSocket(), 5000);
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            // 保持重连
        };
    }

    disconnectWebSocket() {
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        this.isConnected = false;
        this.updateConnectionStatus();
    }

    updateConnectionStatus() {
        const statusEl = document.getElementById('connectionStatus');
        if (this.isConnected) {
            statusEl.textContent = 'WebSocket: 已连接';
            statusEl.className = 'connection-status connection-connected';
        } else {
            statusEl.textContent = 'WebSocket: 未连接';
            statusEl.className = 'connection-status connection-disconnected';
        }
    }

    handleWebSocketMessage(data) {
        // 服务器可能将多条JSON消息合并到一个帧（以\n分隔），这里逐条处理
        const chunks = typeof data === 'string' ? data.split('\n') : [data];
        for (const chunk of chunks) {
            const text = chunk && chunk.trim();
            if (!text) continue;
            try {
                const message = JSON.parse(text);
                switch (message.type) {
                    case 'metrics':
                        console.log('收到指标数据:', message.data.host_id, '指标数量:', message.data.metrics?.length || 0);
                        this.updateMetrics(message.data);
                        break;
                    case 'alert':
                        this.updateAlerts(message.data);
                        break;
                    case 'heartbeat':
                        break;
                    default:
                        // 忽略未知类型
                        break;
                }
            } catch (error) {
                console.error('Parse message error for chunk:', text, error);
            }
        }
    }

    updateMetrics(data) {
        // 存储当前指标数据
        if (data.host_id) {
            this.currentMetrics[data.host_id] = {};
            for (const metric of data.metrics) {
                this.currentMetrics[data.host_id][metric.name] = metric.value;
            }
        }

        // Helper: 根据候选名称获取第一个匹配的指标
        const getMetricByNames = (names) => {
            for (const n of names) {
                const m = data.metrics.find(x => x.name === n);
                if (m) return m;
            }
            return null;
        };

        // 更新CPU使用率（兼容 cpu_usage 与 cpu.usage）
        const cpuMetric = getMetricByNames(['cpu_usage', 'cpu.usage']);
        if (cpuMetric && typeof cpuMetric.value === 'number') {
            document.getElementById('cpuUsage').textContent = `${cpuMetric.value.toFixed(1)}%`;
            this.updateChart('cpuChart', cpuMetric.value);
        }

        // 更新内存使用率（优先 memory_usage；否则用 memory_used/memory_total 计算）
        let memoryUsageMetric = getMetricByNames(['memory_usage', 'memory.usage']);
        if (!memoryUsageMetric) {
            const memUsed = getMetricByNames(['memory_used', 'memory.used']);
            const memTotal = getMetricByNames(['memory_total', 'memory.total']);
            if (memUsed && memTotal && memTotal.value > 0) {
                memoryUsageMetric = { value: (memUsed.value / memTotal.value) * 100 };
            }
        }
        if (memoryUsageMetric && typeof memoryUsageMetric.value === 'number') {
            document.getElementById('memoryUsage').textContent = `${memoryUsageMetric.value.toFixed(1)}%`;
            this.updateChart('memoryChart', memoryUsageMetric.value);
        }

        // 更新网络流量（聚合所有 network_traffic 或 network.bandwidth.*）
        let totalTraffic = 0;
        const trafficMetrics = data.metrics.filter(m => m.name === 'network_traffic' || m.name.startsWith('network.bandwidth.'));
        if (trafficMetrics.length > 0) {
            for (const m of trafficMetrics) {
                // server侧若传的是 B/s 的 network.bandwidth.*，前端统一转成 MB/s 显示
                const val = typeof m.value === 'number' ? (m.name === 'network_traffic' ? m.value : (m.value / (1024 * 1024))) : 0;
                totalTraffic += val;
            }
        } else {
            // 兼容老的 network_rx/tx（KB/s），折算为 MB/s
            const rx = getMetricByNames(['network_rx']);
            const tx = getMetricByNames(['network_tx']);
            if (rx && typeof rx.value === 'number') totalTraffic += rx.value / 1024;
            if (tx && typeof tx.value === 'number') totalTraffic += tx.value / 1024;
        }
        if (totalTraffic > 0) {
            document.getElementById('networkTraffic').textContent = `${totalTraffic.toFixed(1)} MB/s`;
            this.updateChart('networkChart', totalTraffic);
        }

        // 更新TopK排行榜
        this.updateTopK();

        // 详细指标回显（帮助对齐命名差异）
        const allMetricsEl = document.getElementById('allMetrics');
        const metricsCountEl = document.getElementById('metricsCount');
        if (allMetricsEl && Array.isArray(data.metrics)) {
            console.log('更新详细指标显示，指标数量:', data.metrics.length);
            
            // 更新指标计数
            if (metricsCountEl) {
                metricsCountEl.textContent = `(${data.metrics.length})`;
            }
            
            // 格式化指标显示
            const lines = data.metrics.map(m => {
                let val = m.value;
                let unit = m.unit || '';
                
                // 格式化数值显示
                if (typeof val === 'number') {
                    if (val >= 1000000) {
                        val = (val / 1000000).toFixed(2);
                        unit = unit ? `M${unit}` : 'M';
                    } else if (val >= 1000) {
                        val = (val / 1000).toFixed(2);
                        unit = unit ? `K${unit}` : 'K';
                    } else if (val < 0.01 && val > 0) {
                        val = val.toFixed(6);
                    } else {
                        val = val.toFixed(4);
                    }
                }
                
                return `${m.name}: ${val}${unit}`;
            });
            
            // 按指标名称排序，便于查看
            lines.sort();
            
            // 使用textContent而不是innerText以提高性能
            allMetricsEl.textContent = lines.join('\n');
            
            console.log('详细指标显示更新完成，显示行数:', lines.length);
        }
    }

    updateAlerts(alertData) {
        const alertsContainer = document.getElementById('alertsContainer');
        const alertEl = document.createElement('div');
        alertEl.className = `alert-item alert-${alertData.severity}`;
        alertEl.innerHTML = `
            <strong>${alertData.rule_name}</strong>
            <div>${alertData.message}</div>
            <small>${new Date(alertData.timestamp).toLocaleString()}</small>
        `;
        
        alertsContainer.insertBefore(alertEl, alertsContainer.firstChild);
        
        // 限制告警数量
        while (alertsContainer.children.length > 10) {
            alertsContainer.removeChild(alertsContainer.lastChild);
        }
    }

    initCharts() {
        // CPU图表
        const cpuCtx = document.getElementById('cpuChart').getContext('2d');
        this.charts.cpuChart = new Chart(cpuCtx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: 'CPU使用率',
                    data: [],
                    borderColor: '#667eea',
                    backgroundColor: 'rgba(102, 126, 234, 0.1)',
                    tension: 0.4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: {
                        beginAtZero: true,
                        max: 100
                    }
                }
            }
        });

        // 内存图表
        const memoryCtx = document.getElementById('memoryChart').getContext('2d');
        this.charts.memoryChart = new Chart(memoryCtx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: '内存使用率',
                    data: [],
                    borderColor: '#764ba2',
                    backgroundColor: 'rgba(118, 75, 162, 0.1)',
                    tension: 0.4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: {
                        beginAtZero: true,
                        max: 100
                    }
                }
            }
        });

        // 网络图表
        const networkCtx = document.getElementById('networkChart').getContext('2d');
        this.charts.networkChart = new Chart(networkCtx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: '网络流量',
                    data: [],
                    borderColor: '#4CAF50',
                    backgroundColor: 'rgba(76, 175, 80, 0.1)',
                    tension: 0.4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: {
                        beginAtZero: true
                    }
                }
            }
        });

        // 初始化TopK（等待真实数据）
    }

    updateChart(chartName, value) {
        const chart = this.charts[chartName];
        if (!chart) return;

        const now = new Date();
        const timeLabel = now.toLocaleTimeString();

        // 添加新数据
        chart.data.labels.push(timeLabel);
        chart.data.datasets[0].data.push(value);

        // 保持数据点数量
        const maxDataPoints = 20;
        if (chart.data.labels.length > maxDataPoints) {
            chart.data.labels.shift();
            chart.data.datasets[0].data.shift();
        }

        chart.update('none');
    }

    updateTopK() {
        // 先尝试本地排序（因为目前只有一个主机，API可能返回空）
        this.fallbackTopK();
        
        // 同时尝试后端API（用于多主机场景）
        fetch('/api/analysis/topk?metric=cpu_usage&k=5')
            .then(response => response.json())
            .then(data => {
                if (data && data.length > 0) {
                    this.renderTopK(data);
                }
            })
            .catch(error => {
                console.error('获取TopK数据失败:', error);
            });
    }

    renderTopK(topKResults) {
        const topkList = document.getElementById('topkList');
        topkList.innerHTML = '';

        if (topKResults.length === 0) {
            const li = document.createElement('li');
            li.className = 'topk-item';
            li.innerHTML = `<div class="topk-host">暂无数据</div>`;
            topkList.appendChild(li);
        } else {
            topKResults.forEach((item, index) => {
                const li = document.createElement('li');
                li.className = 'topk-item';
                li.innerHTML = `
                    <div style="display: flex; align-items: center;">
                        <div class="topk-rank">${index + 1}</div>
                        <div class="topk-host">${item.host_id || item.HostID}</div>
                    </div>
                    <div class="topk-value">${(item.value || item.Value || 0).toFixed(1)}%</div>
                `;
                topkList.appendChild(li);
            });
        }
    }

    fallbackTopK() {
        // 降级方案：使用前端排序
        const topKData = [];
        
        // 检查是否有CPU数据
        for (const [hostId, metrics] of Object.entries(this.currentMetrics)) {
            if (metrics.cpu_usage !== undefined) {
                topKData.push({
                    host_id: hostId,
                    value: parseFloat(metrics.cpu_usage) || 0
                });
            }
        }
        
        // 如果没有找到数据，尝试从document获取显示的CPU使用率
        if (topKData.length === 0) {
            const cpuElement = document.getElementById('cpuUsage');
            if (cpuElement && cpuElement.textContent) {
                const cpuText = cpuElement.textContent.replace('%', '');
                const cpuValue = parseFloat(cpuText);
                if (!isNaN(cpuValue)) {
                    topKData.push({
                        host_id: 'host-1',
                        value: cpuValue
                    });
                }
            }
        }
        
        topKData.sort((a, b) => b.value - a.value);
        this.renderTopK(topKData.slice(0, 5));
    }


}

// 初始化应用
document.addEventListener('DOMContentLoaded', () => {
    new MonitorApp();
});