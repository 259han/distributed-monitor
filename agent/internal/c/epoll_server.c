#include "epoll_server.h"
#include <sys/time.h>
#include <sys/resource.h>
#include <stdarg.h>
#include <sys/syscall.h>

// TCP选项定义（如果系统没有定义）
#ifndef TCP_KEEPIDLE
#define TCP_KEEPIDLE 4
#endif

#ifndef TCP_KEEPINTVL
#define TCP_KEEPINTVL 5
#endif

#ifndef TCP_KEEPCNT
#define TCP_KEEPCNT 6
#endif

#ifndef TCP_NODELAY
#define TCP_NODELAY 1
#endif

static epoll_server_t* g_server = NULL;
static data_callback_t g_data_callback = NULL;
static connection_callback_t g_connection_callback = NULL;
static error_callback_t g_error_callback = NULL;
static stats_callback_t g_stats_callback = NULL;
static void* g_user_data = NULL;
static int g_log_level = 1;

// Forward declarations for static functions
static int set_nonblocking(int fd);
static int set_socket_options(int fd, epoll_server_t *server);
static void* worker_thread(void *arg);
static void* heartbeat_thread(void *arg);
static void handle_new_connection(epoll_server_t *server);
static void handle_client_data(epoll_server_t *server, int fd);
static void handle_client_disconnect(epoll_server_t *server, int fd);
static int initialize_client_connection(epoll_server_t *server, int client_fd, struct sockaddr_in *addr_ptr);
static void update_connection_stats(epoll_server_t *server, int fd, int bytes_sent, int bytes_received);
static void log_message(int level, const char *format, ...);
static const char* get_error_string(int error_code);

epoll_server_t* epoll_server_create(int port) {
    return epoll_server_create_with_config(port, MAX_CONNECTIONS, MAX_THREADS);
}

epoll_server_t* epoll_server_create_with_config(int port, int max_connections, int thread_count) {
    if (port <= 0 || max_connections <= 0 || thread_count <= 0 || thread_count > MAX_THREADS) {
        return NULL;
    }

    epoll_server_t *server = (epoll_server_t*)malloc(sizeof(epoll_server_t));
    if (!server) {
        return NULL;
    }

    memset(server, 0, sizeof(epoll_server_t));
    server->running = 0;
    server->port = port;
    server->max_connections = max_connections;
    server->thread_count = thread_count;
    
    // 初始化统计信息
    server->stats.start_time = time(NULL);
    pthread_mutex_init(&server->stats_mutex, NULL);
    
    // 默认配置
    server->enable_keepalive = 1;
    server->keepalive_time = 7200;
    server->keepalive_interval = 75;
    server->keepalive_probes = 9;
    server->enable_nagle = 0;
    server->socket_timeout = 30;
    server->max_buffer_size = BUFFER_SIZE;
    server->heartbeat_enabled = 0;

    // 初始化无锁队列
    if (lfq_init(&server->connection_queue, max_connections) != 0) {
        free(server);
        return NULL;
    }

    // 分配连接数组
    server->connections = (connection_t*)malloc(sizeof(connection_t) * max_connections);
    if (!server->connections) {
        lfq_destroy(&server->connection_queue);
        free(server);
        return NULL;
    }
    memset(server->connections, 0, sizeof(connection_t) * max_connections);
    
    // 初始化连接数组
    for (int i = 0; i < max_connections; i++) {
        server->connections[i].fd = -1;
        server->connections[i].state = CONNECTION_STATE_DISCONNECTED;
    }
    
    // 分配线程池
    server->thread_pool = (pthread_t*)malloc(sizeof(pthread_t) * thread_count);
    if (!server->thread_pool) {
        free(server->connections);
        lfq_destroy(&server->connection_queue);
        free(server);
        return NULL;
    }

    // 创建监听socket
    server->server_fd = socket(AF_INET, SOCK_STREAM | SOCK_NONBLOCK, 0);
    if (server->server_fd == -1) {
        free(server->thread_pool);
        free(server->connections);
        lfq_destroy(&server->connection_queue);
        free(server);
        return NULL;
    }

    // 设置socket选项
    if (set_socket_options(server->server_fd, server) != 0) {
        close(server->server_fd);
        free(server->thread_pool);
        free(server->connections);
        lfq_destroy(&server->connection_queue);
        free(server);
        return NULL;
    }

    // 绑定地址
    struct sockaddr_in server_addr;
    memset(&server_addr, 0, sizeof(server_addr));
    server_addr.sin_family = AF_INET;
    server_addr.sin_addr.s_addr = INADDR_ANY;
    server_addr.sin_port = htons(port);

    if (bind(server->server_fd, (struct sockaddr*)&server_addr, sizeof(server_addr)) == -1) {
        close(server->server_fd);
        free(server->thread_pool);
        free(server->connections);
        lfq_destroy(&server->connection_queue);
        free(server);
        return NULL;
    }

    // 开始监听
    if (listen(server->server_fd, BACKLOG) == -1) {
        close(server->server_fd);
        free(server->thread_pool);
        free(server->connections);
        lfq_destroy(&server->connection_queue);
        free(server);
        return NULL;
    }

    // 创建epoll实例
    server->epoll_fd = epoll_create1(0);
    if (server->epoll_fd == -1) {
        close(server->server_fd);
        free(server->thread_pool);
        free(server->connections);
        lfq_destroy(&server->connection_queue);
        free(server);
        return NULL;
    }

    // 添加监听socket到epoll
    struct epoll_event ev;
    ev.events = EPOLLIN | EPOLLET;
    ev.data.fd = server->server_fd;
    if (epoll_ctl(server->epoll_fd, EPOLL_CTL_ADD, server->server_fd, &ev) == -1) {
        close(server->epoll_fd);
        close(server->server_fd);
        free(server->thread_pool);
        free(server->connections);
        lfq_destroy(&server->connection_queue);
        free(server);
        return NULL;
    }

    // 设置文件描述符限制
    struct rlimit lim;
    lim.rlim_cur = 65535;
    lim.rlim_max = 65535;
    setrlimit(RLIMIT_NOFILE, &lim);

    log_message(1, "Epoll server created on port %d with %d max connections", port, max_connections);
    return server;
}

int epoll_server_start(epoll_server_t *server) {
    if (!server || server->running) {
        return EPOLL_SERVER_INVALID_PARAM;
    }

    server->running = 1;
    g_server = server;

    // 创建工作线程池
    for (int i = 0; i < server->thread_count; i++) {
        if (pthread_create(&server->thread_pool[i], NULL, worker_thread, server) != 0) {
            server->running = 0;
            return EPOLL_SERVER_THREAD_ERROR;
        }
    }

    log_message(1, "Epoll server started with %d worker threads", server->thread_count);
    return EPOLL_SERVER_SUCCESS;
}

void epoll_server_stop(epoll_server_t *server) {
    if (!server || !server->running) {
        return;
    }

    server->running = 0;

    // 停止心跳线程
    if (server->heartbeat_enabled) {
        pthread_join(server->heartbeat_thread, NULL);
    }

    // 等待线程池结束
    for (int i = 0; i < server->thread_count; i++) {
        pthread_join(server->thread_pool[i], NULL);
    }

    log_message(1, "Epoll server stopped");
}

void epoll_server_destroy(epoll_server_t *server) {
    if (!server) {
        return;
    }

    if (server->running) {
        epoll_server_stop(server);
    }

    // 关闭所有连接
    for (int i = 0; i < server->max_connections; i++) {
        if (server->connections[i].fd != -1) {
            close(server->connections[i].fd);
        }
    }

    // 关闭epoll和server socket
    if (server->epoll_fd != -1) {
        close(server->epoll_fd);
    }
    if (server->server_fd != -1) {
        close(server->server_fd);
    }

    // 释放资源
    pthread_mutex_destroy(&server->stats_mutex);
    free(server->connections);
    free(server->thread_pool);
    lfq_destroy(&server->connection_queue);
    free(server);
    
    g_server = NULL;
    log_message(1, "Epoll server destroyed");
}

int epoll_server_set_callback(epoll_server_t *server, data_callback_t data_cb, connection_callback_t conn_cb, void *user_data) {
    if (!server) {
        return EPOLL_SERVER_INVALID_PARAM;
    }

    g_data_callback = data_cb;
    g_connection_callback = conn_cb;
    g_user_data = user_data;
    server->data_callback = data_cb;
    server->connection_callback = conn_cb;
    server->user_data = user_data;

    return EPOLL_SERVER_SUCCESS;
}

int epoll_server_set_error_callback(epoll_server_t *server, error_callback_t error_cb) {
    if (!server) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    g_error_callback = error_cb;
    server->error_callback = error_cb;
    return EPOLL_SERVER_SUCCESS;
}

int epoll_server_set_stats_callback(epoll_server_t *server, stats_callback_t stats_cb) {
    if (!server) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    g_stats_callback = stats_cb;
    server->stats_callback = stats_cb;
    return EPOLL_SERVER_SUCCESS;
}

int epoll_server_send_data(epoll_server_t *server, int fd, const char *data, int len) {
    if (!server || fd < 0 || !data || len <= 0) {
        return EPOLL_SERVER_INVALID_PARAM;
    }

    int bytes_sent = send(fd, data, len, MSG_NOSIGNAL);
    if (bytes_sent > 0) {
        update_connection_stats(server, fd, bytes_sent, 0);
    }
    
    return bytes_sent;
}

int epoll_server_send_data_to_all(epoll_server_t *server, const char *data, int len) {
    if (!server || !data || len <= 0) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    int success_count = 0;
    for (int i = 0; i < server->max_connections; i++) {
        if (server->connections[i].fd != -1 && 
            server->connections[i].state == CONNECTION_STATE_CONNECTED) {
            if (epoll_server_send_data(server, server->connections[i].fd, data, len) > 0) {
                success_count++;
            }
        }
    }
    
    return success_count;
}

int epoll_server_send_data_except(epoll_server_t *server, int except_fd, const char *data, int len) {
    if (!server || !data || len <= 0) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    int success_count = 0;
    for (int i = 0; i < server->max_connections; i++) {
        if (server->connections[i].fd != -1 && 
            server->connections[i].fd != except_fd &&
            server->connections[i].state == CONNECTION_STATE_CONNECTED) {
            if (epoll_server_send_data(server, server->connections[i].fd, data, len) > 0) {
                success_count++;
            }
        }
    }
    
    return success_count;
}

static int set_nonblocking(int fd) {
    int flags = fcntl(fd, F_GETFL, 0);
    if (flags == -1) {
        return -1;
    }
    return fcntl(fd, F_SETFL, flags | O_NONBLOCK);
}

static int set_socket_options(int fd, epoll_server_t *server) {
    int opt = 1;
    
    // 设置地址重用
    if (setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt)) == -1) {
        return -1;
    }
    
    // 设置端口重用
    if (setsockopt(fd, SOL_SOCKET, SO_REUSEPORT, &opt, sizeof(opt)) == -1) {
        return -1;
    }
    
    // 设置Keepalive
    if (server->enable_keepalive) {
        if (setsockopt(fd, SOL_SOCKET, SO_KEEPALIVE, &opt, sizeof(opt)) == -1) {
            return -1;
        }
        
        // 设置TCP keepalive参数
        if (setsockopt(fd, IPPROTO_TCP, TCP_KEEPIDLE, &server->keepalive_time, sizeof(server->keepalive_time)) == -1) {
            return -1;
        }
        
        if (setsockopt(fd, IPPROTO_TCP, TCP_KEEPINTVL, &server->keepalive_interval, sizeof(server->keepalive_interval)) == -1) {
            return -1;
        }
        
        if (setsockopt(fd, IPPROTO_TCP, TCP_KEEPCNT, &server->keepalive_probes, sizeof(server->keepalive_probes)) == -1) {
            return -1;
        }
    }
    
    // 设置Nagle算法
    if (setsockopt(fd, IPPROTO_TCP, TCP_NODELAY, &server->enable_nagle, sizeof(server->enable_nagle)) == -1) {
        return -1;
    }
    
    // 设置发送超时
    struct timeval timeout;
    timeout.tv_sec = server->socket_timeout;
    timeout.tv_usec = 0;
    
    if (setsockopt(fd, SOL_SOCKET, SO_SNDTIMEO, &timeout, sizeof(timeout)) == -1) {
        return -1;
    }
    
    if (setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &timeout, sizeof(timeout)) == -1) {
        return -1;
    }
    
    return 0;
}

static void update_connection_stats(epoll_server_t *server, int fd, int bytes_sent, int bytes_received) {
    pthread_mutex_lock(&server->stats_mutex);
    
    for (int i = 0; i < server->max_connections; i++) {
        if (server->connections[i].fd == fd) {
            server->connections[i].last_activity = time(NULL);
            if (bytes_sent > 0) {
                server->connections[i].bytes_sent += bytes_sent;
                server->connections[i].messages_sent++;
                server->stats.total_bytes_sent += bytes_sent;
            }
            if (bytes_received > 0) {
                server->connections[i].bytes_received += bytes_received;
                server->connections[i].messages_received++;
                server->stats.total_bytes_received += bytes_received;
                server->stats.total_messages++;
            }
            break;
        }
    }
    
    pthread_mutex_unlock(&server->stats_mutex);
}

static void log_message(int level, const char *format, ...) {
    if (level > g_log_level) {
        return;
    }
    
    va_list args;
    va_start(args, format);
    
    char timestamp[64];
    time_t now = time(NULL);
    strftime(timestamp, sizeof(timestamp), "%Y-%m-%d %H:%M:%S", localtime(&now));
    
    printf("[%s] [EPOLL] ", timestamp);
    vprintf(format, args);
    printf("\n");
    
    va_end(args);
}

static const char* get_error_string(int error_code) {
    switch (error_code) {
        case EPOLL_SERVER_SUCCESS: return "Success";
        case EPOLL_SERVER_ERROR: return "General error";
        case EPOLL_SERVER_INVALID_PARAM: return "Invalid parameter";
        case EPOLL_SERVER_MEMORY_ERROR: return "Memory error";
        case EPOLL_SERVER_SOCKET_ERROR: return "Socket error";
        case EPOLL_SERVER_BIND_ERROR: return "Bind error";
        case EPOLL_SERVER_LISTEN_ERROR: return "Listen error";
        case EPOLL_SERVER_EPOLL_ERROR: return "Epoll error";
        case EPOLL_SERVER_THREAD_ERROR: return "Thread error";
        default: return "Unknown error";
    }
}

static void* worker_thread(void *arg) {
    epoll_server_t *server = (epoll_server_t*)arg;
    struct epoll_event events[MAX_EVENTS];
    pid_t tid = syscall(SYS_gettid);
    
    log_message(2, "Worker thread started, tid: %d", tid);

    while (server->running) {
        // 初始化新连接（从无锁队列中取出并完成设置）
        lfq_connection_t new_conn;
        while (lfq_dequeue(&server->connection_queue, &new_conn) == 0) {
            int client_fd = new_conn.fd;
            struct sockaddr_in *addr_ptr = (struct sockaddr_in*)new_conn.data;

            if (!addr_ptr) {
                log_message(1, "Dequeued connection without address data for fd %d", client_fd);
                close(client_fd);
                continue;
            }

            // 使用辅助函数完成连接初始化
            if (initialize_client_connection(server, client_fd, addr_ptr) != 0) {
                log_message(1, "Failed to initialize connection for fd %d", client_fd);
                close(client_fd);
            }
            
            // 释放地址副本
            free(addr_ptr);
        }

        int nfds = epoll_wait(server->epoll_fd, events, MAX_EVENTS, 100);
        if (nfds == -1) {
            if (errno == EINTR) {
                continue;
            }
            log_message(1, "Epoll wait error: %s", strerror(errno));
            break;
        }

        for (int i = 0; i < nfds; i++) {
            int fd = events[i].data.fd;

            if (fd == server->server_fd) {
                handle_new_connection(server);
            } else {
                if (events[i].events & EPOLLIN) {
                    handle_client_data(server, fd);
                }
                if (events[i].events & (EPOLLERR | EPOLLHUP | EPOLLRDHUP)) {
                    handle_client_disconnect(server, fd);
                }
            }
        }
    }

    log_message(2, "Worker thread ended, tid: %d", tid);
    return NULL;
}

static void* heartbeat_thread(void *arg) {
    epoll_server_t *server = (epoll_server_t*)arg;
    
    log_message(2, "Heartbeat thread started");
    
    while (server->running) {
        sleep(HEARTBEAT_INTERVAL);
        
        if (!server->running) {
            break;
        }
        
        // 检查不活跃连接
        int inactive_count = epoll_server_check_inactive_connections(server, HEARTBEAT_INTERVAL * 2);
        if (inactive_count > 0) {
            log_message(2, "Closed %d inactive connections", inactive_count);
        }
        
        // 清理统计信息
        if (g_stats_callback) {
            pthread_mutex_lock(&server->stats_mutex);
            g_stats_callback(&server->stats, server->user_data);
            pthread_mutex_unlock(&server->stats_mutex);
        }
    }
    
    log_message(2, "Heartbeat thread ended");
    return NULL;
}

static void handle_new_connection(epoll_server_t *server) {
    while (1) {
        struct sockaddr_in client_addr;
        socklen_t client_len = sizeof(client_addr);
        int client_fd = accept(server->server_fd, (struct sockaddr*)&client_addr, &client_len);

        if (client_fd == -1) {
            if (errno == EAGAIN || errno == EWOULDBLOCK) {
                // Edge-triggered: all incoming connections accepted
                break;
            }
            log_message(1, "Accept error: %s", strerror(errno));
            break;
        }

        // 复制客户端地址，避免入队栈指针
        struct sockaddr_in *addr_copy = (struct sockaddr_in*)malloc(sizeof(struct sockaddr_in));
        if (!addr_copy) {
            log_message(1, "Memory allocation failed for client address, closing fd %d", client_fd);
            close(client_fd);
            continue;
        }
        *addr_copy = client_addr;

        // 入队等待工作线程完成初始化
        if (lfq_enqueue(&server->connection_queue, client_fd, addr_copy) != 0) {
            log_message(1, "Connection queue full or limit reached, rejecting fd %d", client_fd);
            close(client_fd);
            free(addr_copy);
            continue;
        }
    }
}

static void handle_client_data(epoll_server_t *server, int fd) {
    char buffer[BUFFER_SIZE];
    int bytes_read = recv(fd, buffer, BUFFER_SIZE - 1, 0);
    
    if (bytes_read <= 0) {
        if (bytes_read == 0) {
            log_message(2, "Client (fd: %d) disconnected gracefully", fd);
        } else {
            log_message(2, "Recv error on fd %d: %s", fd, strerror(errno));
        }
        handle_client_disconnect(server, fd);
        return;
    }

    buffer[bytes_read] = '\0';

    // 查找对应的连接
    for (int i = 0; i < server->max_connections; i++) {
        if (server->connections[i].fd == fd) {
            connection_t *conn = &server->connections[i];
            
            // 追加到缓冲区
            int remaining = server->max_buffer_size - conn->buffer_len - 1;
            if (remaining > 0) {
                int copy_len = bytes_read < remaining ? bytes_read : remaining;
                memcpy(conn->buffer + conn->buffer_len, buffer, copy_len);
                conn->buffer_len += copy_len;
                conn->buffer[conn->buffer_len] = '\0';
            } else {
                log_message(1, "Buffer overflow for connection %d, truncating data", fd);
            }

            // 更新统计信息
            update_connection_stats(server, fd, 0, bytes_read);

            // 调用回调函数
            if (g_data_callback) {
                g_data_callback(fd, buffer, bytes_read, g_user_data);
            }
            break;
        }
    }
}

static void handle_client_disconnect(epoll_server_t *server, int fd) {
    // 查找连接信息
    connection_t *conn = NULL;
    
    for (int i = 0; i < server->max_connections; i++) {
        if (server->connections[i].fd == fd) {
            conn = &server->connections[i];
            break;
        }
    }
    
    if (!conn) {
        return;
    }
    
    log_message(2, "Client %s:%d (fd: %d) disconnected", conn->client_ip, conn->client_port, fd);
    
    // 调用连接回调
    if (g_connection_callback) {
        g_connection_callback(fd, 0, g_user_data);
    }
    
    // 调用错误回调
    if (g_error_callback) {
        g_error_callback(fd, CONNECTION_STATE_DISCONNECTED, "Client disconnected", g_user_data);
    }
    
    // 从epoll移除
    epoll_ctl(server->epoll_fd, EPOLL_CTL_DEL, fd, NULL);
    close(fd);

    // 清理连接信息
    conn->fd = -1;
    conn->state = CONNECTION_STATE_DISCONNECTED;
    conn->buffer_len = 0;
    
    // 更新统计信息
    pthread_mutex_lock(&server->stats_mutex);
    server->stats.active_connections--;
    pthread_mutex_unlock(&server->stats_mutex);
}

// 连接管理函数
int epoll_server_close_connection(epoll_server_t *server, int fd) {
    if (!server || fd < 0) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    handle_client_disconnect(server, fd);
    return EPOLL_SERVER_SUCCESS;
}

int epoll_server_get_connection_info(epoll_server_t *server, int fd, connection_t *info) {
    if (!server || !info || fd < 0) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    for (int i = 0; i < server->max_connections; i++) {
        if (server->connections[i].fd == fd) {
            memcpy(info, &server->connections[i], sizeof(connection_t));
            return EPOLL_SERVER_SUCCESS;
        }
    }
    
    return EPOLL_SERVER_ERROR;
}

int epoll_server_is_connection_active(epoll_server_t *server, int fd) {
    if (!server || fd < 0) {
        return 0;
    }
    
    for (int i = 0; i < server->max_connections; i++) {
        if (server->connections[i].fd == fd) {
            return server->connections[i].state == CONNECTION_STATE_CONNECTED;
        }
    }
    
    return 0;
}

// 配置设置函数
int epoll_server_set_keepalive(epoll_server_t *server, int enable, int time, int interval, int probes) {
    if (!server) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    server->enable_keepalive = enable;
    server->keepalive_time = time;
    server->keepalive_interval = interval;
    server->keepalive_probes = probes;
    
    return EPOLL_SERVER_SUCCESS;
}

int epoll_server_set_nagle(epoll_server_t *server, int enable) {
    if (!server) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    server->enable_nagle = enable;
    return EPOLL_SERVER_SUCCESS;
}

int epoll_server_set_timeout(epoll_server_t *server, int timeout) {
    if (!server) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    server->socket_timeout = timeout;
    return EPOLL_SERVER_SUCCESS;
}

int epoll_server_set_buffer_size(epoll_server_t *server, int size) {
    if (!server || size <= 0) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    server->max_buffer_size = size;
    return EPOLL_SERVER_SUCCESS;
}

// 统计信息函数
int epoll_server_get_stats(epoll_server_t *server, server_stats_t *stats) {
    if (!server || !stats) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    pthread_mutex_lock(&server->stats_mutex);
    memcpy(stats, &server->stats, sizeof(server_stats_t));
    pthread_mutex_unlock(&server->stats_mutex);
    
    return EPOLL_SERVER_SUCCESS;
}

int epoll_server_reset_stats(epoll_server_t *server) {
    if (!server) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    pthread_mutex_lock(&server->stats_mutex);
    memset(&server->stats, 0, sizeof(server_stats_t));
    server->stats.start_time = time(NULL);
    pthread_mutex_unlock(&server->stats_mutex);
    
    return EPOLL_SERVER_SUCCESS;
}

void epoll_server_print_stats(epoll_server_t *server) {
    if (!server) {
        return;
    }
    
    server_stats_t stats;
    if (epoll_server_get_stats(server, &stats) == EPOLL_SERVER_SUCCESS) {
        time_t now = time(NULL);
        double uptime = difftime(now, stats.start_time);
        
        printf("=== Epoll Server Statistics ===\n");
        printf("Uptime: %.2f seconds\n", uptime);
        printf("Total connections: %lu\n", stats.total_connections);
        printf("Active connections: %lu\n", stats.active_connections);
        printf("Total bytes sent: %lu\n", stats.total_bytes_sent);
        printf("Total bytes received: %lu\n", stats.total_bytes_received);
        printf("Total messages: %lu\n", stats.total_messages);
        printf("Average response time: %.2f ms\n", stats.avg_response_time);
        printf("===============================\n");
    }
}

// 心跳检测函数
int epoll_server_enable_heartbeat(epoll_server_t *server, int enable) {
    if (!server) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    if (enable && !server->heartbeat_enabled) {
        server->heartbeat_enabled = 1;
        if (pthread_create(&server->heartbeat_thread, NULL, heartbeat_thread, server) != 0) {
            server->heartbeat_enabled = 0;
            return EPOLL_SERVER_THREAD_ERROR;
        }
    } else if (!enable && server->heartbeat_enabled) {
        server->heartbeat_enabled = 0;
        pthread_join(server->heartbeat_thread, NULL);
    }
    
    return EPOLL_SERVER_SUCCESS;
}

int epoll_server_check_inactive_connections(epoll_server_t *server, int timeout_seconds) {
    if (!server || timeout_seconds <= 0) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    time_t now = time(NULL);
    int closed_count = 0;
    
    for (int i = 0; i < server->max_connections; i++) {
        if (server->connections[i].fd != -1 && 
            server->connections[i].state == CONNECTION_STATE_CONNECTED) {
            
            double inactive_time = difftime(now, server->connections[i].last_activity);
            if (inactive_time > timeout_seconds) {
                log_message(2, "Closing inactive connection %s:%d (fd: %d), inactive for %.2f seconds",
                           server->connections[i].client_ip, server->connections[i].client_port,
                           server->connections[i].fd, inactive_time);
                epoll_server_close_connection(server, server->connections[i].fd);
                closed_count++;
            }
        }
    }
    
    return closed_count;
}

// 工具函数
const char* epoll_server_strerror(int error_code) {
    return get_error_string(error_code);
}

int epoll_server_get_fd_count(epoll_server_t *server) {
    if (!server) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    return server->stats.active_connections;
}

int epoll_server_broadcast_message(epoll_server_t *server, const message_t *msg) {
    if (!server || !msg || !msg->data || msg->len <= 0) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    return epoll_server_send_data_to_all(server, msg->data, msg->len);
}

// 信号处理函数
void epoll_server_setup_signal_handlers(epoll_server_t *server) {
    if (!server) {
        return;
    }
    
    signal(SIGINT, epoll_server_handle_signal);
    signal(SIGTERM, epoll_server_handle_signal);
    signal(SIGPIPE, SIG_IGN);
}

void epoll_server_handle_signal(int sig) {
    log_message(1, "Received signal %d, shutting down...", sig);
    if (g_server) {
        epoll_server_stop(g_server);
    }
}

// 内存管理函数
int epoll_server_resize_connections(epoll_server_t *server, int new_size) {
    if (!server || new_size <= 0 || new_size <= server->max_connections) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    connection_t *new_connections = (connection_t*)realloc(server->connections, sizeof(connection_t) * new_size);
    if (!new_connections) {
        return EPOLL_SERVER_MEMORY_ERROR;
    }
    
    // 初始化新增的连接槽位
    for (int i = server->max_connections; i < new_size; i++) {
        new_connections[i].fd = -1;
        new_connections[i].state = CONNECTION_STATE_DISCONNECTED;
    }
    
    server->connections = new_connections;
    server->max_connections = new_size;
    
    log_message(1, "Resized connections array to %d", new_size);
    return EPOLL_SERVER_SUCCESS;
}

int epoll_server_cleanup_inactive_connections(epoll_server_t *server) {
    if (!server) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    return epoll_server_check_inactive_connections(server, 300); // 5分钟超时
}

// 线程池管理函数
int epoll_server_set_thread_count(epoll_server_t *server, int count) {
    if (!server || count <= 0 || count > MAX_THREADS) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    if (server->running) {
        return EPOLL_SERVER_ERROR;
    }
    
    pthread_t *new_thread_pool = (pthread_t*)realloc(server->thread_pool, sizeof(pthread_t) * count);
    if (!new_thread_pool) {
        return EPOLL_SERVER_MEMORY_ERROR;
    }
    
    server->thread_pool = new_thread_pool;
    server->thread_count = count;
    
    return EPOLL_SERVER_SUCCESS;
}

int epoll_server_get_thread_count(epoll_server_t *server) {
    if (!server) {
        return EPOLL_SERVER_INVALID_PARAM;
    }
    
    return server->thread_count;
}

// 连接初始化辅助函数
static int initialize_client_connection(epoll_server_t *server, int client_fd, struct sockaddr_in *addr_ptr) {
    // 设置为非阻塞
    if (set_nonblocking(client_fd) == -1) {
        log_message(1, "Failed to set nonblocking for fd %d", client_fd);
        return -1;
    }

    // 设置socket选项
    if (set_socket_options(client_fd, server) != 0) {
        log_message(1, "Failed to set socket options for fd %d", client_fd);
        return -1;
    }

    // 添加到epoll
    struct epoll_event ev;
    ev.events = EPOLLIN | EPOLLET | EPOLLRDHUP;
    ev.data.fd = client_fd;
    if (epoll_ctl(server->epoll_fd, EPOLL_CTL_ADD, client_fd, &ev) == -1) {
        log_message(1, "Failed to add fd %d to epoll", client_fd);
        return -1;
    }

    // 保存连接信息
    connection_t *conn = NULL;
    for (int i = 0; i < server->max_connections; i++) {
        if (server->connections[i].fd == -1) {
            conn = &server->connections[i];
            conn->fd = client_fd;
            conn->addr = *addr_ptr;
            conn->buffer_len = 0;
            conn->state = CONNECTION_STATE_CONNECTED;
            conn->connect_time = time(NULL);
            conn->last_activity = conn->connect_time;
            conn->bytes_sent = 0;
            conn->bytes_received = 0;
            conn->messages_sent = 0;
            conn->messages_received = 0;
            conn->client_port = ntohs(addr_ptr->sin_port);
            strncpy(conn->client_ip, inet_ntoa(addr_ptr->sin_addr), INET_ADDRSTRLEN - 1);
            conn->client_ip[INET_ADDRSTRLEN - 1] = '\0';
            break;
        }
    }

    if (!conn) {
        log_message(1, "No available connection slot for fd %d", client_fd);
        epoll_ctl(server->epoll_fd, EPOLL_CTL_DEL, client_fd, NULL);
        return -1;
    }

    // 更新统计信息
    pthread_mutex_lock(&server->stats_mutex);
    server->stats.total_connections++;
    server->stats.active_connections++;
    pthread_mutex_unlock(&server->stats_mutex);

    log_message(2, "Initialized new connection (fd: %d)", client_fd);

    // 调用连接回调
    if (g_connection_callback) {
        g_connection_callback(client_fd, 1, g_user_data);
    }

    return 0;
}

// 日志记录函数
void epoll_server_set_log_level(int level) {
    g_log_level = level;
}

void epoll_server_log(const char *format, ...) {
    va_list args;
    va_start(args, format);
    log_message(1, format, args);
    va_end(args);
}