class MonitorApp {
    constructor() {
        this.ws = null;
        this.charts = {};
        this.isConnected = false;
        this.authToken = localStorage.getItem('authToken');
        this.refreshToken = localStorage.getItem('refreshToken');
        
        this.init();
    }

    init() {
        this.bindEvents();
        this.checkAuth();
        this.initCharts();
    }

    bindEvents() {
        // 登录表单事件
        document.getElementById('loginFormElement').addEventListener('submit', (e) => {
            e.preventDefault();
            this.handleLogin();
        });

        // 登录按钮事件
        document.getElementById('loginBtn').addEventListener('click', () => {
            this.showLoginForm();
        });

        // 登出按钮事件
        document.getElementById('logoutBtn').addEventListener('click', () => {
            this.handleLogout();
        });
    }

    checkAuth() {
        if (this.authToken) {
            this.showDashboard();
            this.connectWebSocket();
            this.updateUserInfo();
        } else {
            this.showLoginForm();
        }
    }

    async handleLogin() {
        const username = document.getElementById('username').value;
        const password = document.getElementById('password').value;

        try {
            const response = await fetch('/api/auth/login', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ username, password })
            });

            if (response.ok) {
                const data = await response.json();
                this.authToken = data.token;
                localStorage.setItem('authToken', this.authToken);
                this.showDashboard();
                this.connectWebSocket();
                this.updateUserInfo();
            } else {
                alert('登录失败，请检查用户名和密码');
            }
        } catch (error) {
            console.error('Login error:', error);
            alert('登录失败，请稍后重试');
        }
    }

    async handleLogout() {
        try {
            await fetch('/api/auth/logout', {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${this.authToken}`
                }
            });
        } catch (error) {
            console.error('Logout error:', error);
        }

        this.authToken = null;
        localStorage.removeItem('authToken');
        localStorage.removeItem('refreshToken');
        this.disconnectWebSocket();
        this.showLoginForm();
    }

    showLoginForm() {
        document.getElementById('loginForm').classList.remove('hidden');
        document.getElementById('dashboard').classList.add('hidden');
        document.getElementById('loginBtn').classList.remove('hidden');
        document.getElementById('logoutBtn').classList.add('hidden');
    }

    showDashboard() {
        document.getElementById('loginForm').classList.add('hidden');
        document.getElementById('dashboard').classList.remove('hidden');
        document.getElementById('loginBtn').classList.add('hidden');
        document.getElementById('logoutBtn').classList.remove('hidden');
    }

    async updateUserInfo() {
        try {
            const response = await fetch('/api/auth/profile', {
                headers: {
                    'Authorization': `Bearer ${this.authToken}`
                }
            });

            if (response.ok) {
                const data = await response.json();
                document.getElementById('userInfo').textContent = `欢迎, ${data.user_id} (${data.role})`;
                document.getElementById('userInfo').classList.remove('hidden');
            }
        } catch (error) {
            console.error('Get profile error:', error);
        }
    }

    connectWebSocket() {
        if (!this.authToken) return;

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

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
        try {
            const message = JSON.parse(data);
            
            switch (message.type) {
                case 'metrics':
                    this.updateMetrics(message.data);
                    break;
                case 'alert':
                    this.updateAlerts(message.data);
                    break;
                case 'heartbeat':
                    // 心跳消息，忽略
                    break;
                default:
                    console.log('Unknown message type:', message.type);
            }
        } catch (error) {
            console.error('Parse message error:', error);
        }
    }

    updateMetrics(data) {
        // 更新CPU使用率
        const cpuMetric = data.metrics.find(m => m.name === 'cpu_usage');
        if (cpuMetric) {
            document.getElementById('cpuUsage').textContent = `${cpuMetric.value.toFixed(1)}%`;
            this.updateChart('cpuChart', cpuMetric.value);
        }

        // 更新内存使用率
        const memoryMetric = data.metrics.find(m => m.name === 'memory_usage');
        if (memoryMetric) {
            document.getElementById('memoryUsage').textContent = `${memoryMetric.value.toFixed(1)}%`;
            this.updateChart('memoryChart', memoryMetric.value);
        }

        // 更新网络流量
        const networkMetric = data.metrics.find(m => m.name === 'network_traffic');
        if (networkMetric) {
            document.getElementById('networkTraffic').textContent = `${networkMetric.value.toFixed(1)} MB/s`;
            this.updateChart('networkChart', networkMetric.value);
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

        // 模拟TopK数据
        this.updateTopK();
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
        // 模拟TopK数据
        const topKData = [
            { host: 'host-1', value: 85.2 },
            { host: 'host-2', value: 72.1 },
            { host: 'host-3', value: 68.9 },
            { host: 'host-4', value: 45.3 },
            { host: 'host-5', value: 32.7 }
        ];

        const topkList = document.getElementById('topkList');
        topkList.innerHTML = '';

        topKData.forEach((item, index) => {
            const li = document.createElement('li');
            li.className = 'topk-item';
            li.innerHTML = `
                <div style="display: flex; align-items: center;">
                    <div class="topk-rank">${index + 1}</div>
                    <div class="topk-host">${item.host}</div>
                </div>
                <div class="topk-value">${item.value.toFixed(1)}%</div>
            `;
            topkList.appendChild(li);
        });

        // 定期更新
        setTimeout(() => this.updateTopK(), 30000);
    }
}

// 初始化应用
document.addEventListener('DOMContentLoaded', () => {
    new MonitorApp();
});