#ifndef EPOLL_SERVER_ENHANCED_H
#define EPOLL_SERVER_ENHANCED_H

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/socket.h>
#include <sys/epoll.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <fcntl.h>
#include <errno.h>
#include <pthread.h>
#include <time.h>
#include <stdint.h>
#include <sys/time.h>
#include <sys/resource.h>
#include <signal.h>
#include <stdarg.h>

// 增强配置
#define MAX_EVENTS 1024
#define MAX_CONNECTIONS 10000
#define BUFFER_SIZE 8192
#define DEFAULT_PORT 8080
#define BACKLOG 1024
#define MAX_THREADS 8
#define HEARTBEAT_INTERVAL 30

// 错误码
#define EPOLL_ENHANCED_SUCCESS 0
#define EPOLL_ENHANCED_ERROR -1
#define EPOLL_ENHANCED_INVALID_PARAM -2
#define EPOLL_ENHANCED_MEMORY_ERROR -3
#define EPOLL_ENHANCED_SOCKET_ERROR -4
#define EPOLL_ENHANCED_BIND_ERROR -5
#define EPOLL_ENHANCED_LISTEN_ERROR -6
#define EPOLL_ENHANCED_EPOLL_ERROR -7
#define EPOLL_ENHANCED_THREAD_ERROR -8

// 日志级别
#define EPOLL_LOG_ERROR 0
#define EPOLL_LOG_WARN 1
#define EPOLL_LOG_INFO 2
#define EPOLL_LOG_DEBUG 3

// 连接状态
#define EPOLL_CONN_CONNECTED 1
#define EPOLL_CONN_DISCONNECTED 0
#define EPOLL_CONN_ERROR -1

// 连接信息结构
typedef struct {
    int fd;
    struct sockaddr_in addr;
    char buffer[BUFFER_SIZE];
    int buffer_len;
    int state;
    time_t connect_time;
    time_t last_activity;
    uint64_t bytes_sent;
    uint64_t bytes_received;
    uint64_t messages_sent;
    uint64_t messages_received;
    char client_ip[INET_ADDRSTRLEN];
    int client_port;
    int keepalive_enabled;
} epoll_connection_t;

// 服务器配置结构
typedef struct {
    int port;
    int max_connections;
    int thread_pool_size;
    int backlog;
    int enable_reuse_port;
    int enable_keepalive;
    int keepalive_time;
    int keepalive_interval;
    int keepalive_probes;
    int enable_nagle;
    int socket_timeout;
    int buffer_size;
    int log_level;
    int enable_stats;
    int enable_heartbeat;
    int heartbeat_interval;
} epoll_server_config_t;

// 统计信息结构
typedef struct {
    uint64_t total_connections;
    uint64_t active_connections;
    uint64_t total_bytes_sent;
    uint64_t total_bytes_received;
    uint64_t total_messages;
    time_t start_time;
    double avg_response_time;
    uint64_t connection_errors;
    uint64_t read_errors;
    uint64_t write_errors;
    uint64_t timeout_errors;
} epoll_server_stats_t;

// 消息结构
typedef struct {
    int fd;
    char *data;
    size_t len;
    struct sockaddr_in addr;
    time_t timestamp;
} epoll_message_t;

// 服务器结构
typedef struct {
    int epoll_fd;
    int server_fd;
    epoll_connection_t *connections;
    int connection_count;
    int max_connections;
    pthread_t *thread_pool;
    int thread_count;
    int running;
    int port;
    
    // 配置
    epoll_server_config_t config;
    
    // 回调函数
    data_callback_t data_callback;
    connection_callback_t connection_callback;
    error_callback_t error_callback;
    stats_callback_t stats_callback;
    void *user_data;
    
    // 统计信息
    epoll_server_stats_t stats;
    pthread_mutex_t stats_mutex;
    
    // 心跳检测
    pthread_t heartbeat_thread;
    int heartbeat_enabled;
    
    // 日志系统
    int log_level;
    pthread_mutex_t log_mutex;
    
    // 内存管理
    int memory_pool_enabled;
    void *memory_pool;
    size_t memory_pool_size;
    
    // 信号处理
    int signal_handling_enabled;
    struct sigaction old_sigint_action;
    struct sigaction old_sigterm_action;
} epoll_server_t;

// 错误回调函数类型
typedef void (*error_callback_t)(int fd, int error_code, const char *error_msg, void *user_data);
typedef void (*stats_callback_t)(const epoll_server_stats_t *stats, void *user_data);

// 基础函数
epoll_server_t* epoll_server_create(int port);
epoll_server_t* epoll_server_create_with_config(const epoll_server_config_t *config);
int epoll_server_start(epoll_server_t *server);
void epoll_server_stop(epoll_server_t *server);
void epoll_server_destroy(epoll_server_t *server);

// 配置管理
int epoll_server_get_config(epoll_server_t *server, epoll_server_config_t *config);
int epoll_server_set_config(epoll_server_t *server, const epoll_server_config_t *config);
int epoll_server_set_log_level(epoll_server_t *server, int level);
int epoll_server_set_buffer_size(epoll_server_t *server, int size);
int epoll_server_set_timeout(epoll_server_t *server, int timeout);
int epoll_server_set_keepalive(epoll_server_t *server, int enable, int time, int interval, int probes);
int epoll_server_set_nagle(epoll_server_t *server, int enable);

// 统计信息
int epoll_server_get_stats(epoll_server_t *server, epoll_server_stats_t *stats);
int epoll_server_reset_stats(epoll_server_t *server);
void epoll_server_print_stats(epoll_server_t *server);
int epoll_server_enable_stats(epoll_server_t *server, int enable);

// 回调设置
int epoll_server_set_data_callback(epoll_server_t *server, data_callback_t callback);
int epoll_server_set_connection_callback(epoll_server_t *server, connection_callback_t callback);
int epoll_server_set_error_callback(epoll_server_t *server, error_callback_t callback);
int epoll_server_set_stats_callback(epoll_server_t *server, stats_callback_t callback);
int epoll_server_set_user_data(epoll_server_t *server, void *user_data);

// 数据操作
int epoll_server_send_data(epoll_server_t *server, int fd, const char *data, size_t len);
int epoll_server_send_to_client(epoll_server_t *server, int fd, const char *data, size_t len);
int epoll_server_broadcast(epoll_server_t *server, const char *data, size_t len);
int epoll_server_broadcast_except(epoll_server_t *server, int except_fd, const char *data, size_t len);
int epoll_server_disconnect_client(epoll_server_t *server, int fd);
int epoll_server_send_message(epoll_server_t *server, const epoll_message_t *msg);

// 连接管理
int epoll_server_get_connection_info(epoll_server_t *server, int fd, epoll_connection_t *info);
int epoll_server_is_connection_active(epoll_server_t *server, int fd);
int epoll_server_get_connection_count(epoll_server_t *server);
int epoll_server_get_active_connections(epoll_server_t *server);
int epoll_server_cleanup_inactive_connections(epoll_server_t *server, int timeout_seconds);

// 心跳检测
int epoll_server_enable_heartbeat(epoll_server_t *server, int enable);
int epoll_server_set_heartbeat_interval(epoll_server_t *server, int interval);
int epoll_server_check_inactive_connections(epoll_server_t *server, int timeout_seconds);

// 线程管理
int epoll_server_set_thread_count(epoll_server_t *server, int count);
int epoll_server_get_thread_count(epoll_server_t *server);
int epoll_server_resize_thread_pool(epoll_server_t *server, int new_size);

// 内存管理
int epoll_server_enable_memory_pool(epoll_server_t *server, int enable);
int epoll_server_set_memory_pool_size(epoll_server_t *server, size_t size);
void* epoll_server_memory_pool_alloc(epoll_server_t *server, size_t size);
void epoll_server_memory_pool_free(epoll_server_t *server, void *ptr);

// 信号处理
int epoll_server_setup_signal_handlers(epoll_server_t *server);
int epoll_server_enable_signal_handling(epoll_server_t *server, int enable);
void epoll_server_handle_signal(int sig);

// 工具函数
const char* epoll_server_strerror(int error_code);
int epoll_server_get_fd_count(epoll_server_t *server);
int epoll_server_is_running(epoll_server_t *server);
int epoll_server_get_port(epoll_server_t *server);

// 调试和日志
void epoll_server_enable_debug(epoll_server_t *server, int enable);
void epoll_server_log(epoll_server_t *server, int level, const char *format, ...);
void epoll_server_dump_connections(epoll_server_t *server);

// 性能监控
int epoll_server_get_performance_metrics(epoll_server_t *server, double *cpu_usage, double *memory_usage);
int epoll_server_set_performance_monitoring(epoll_server_t *server, int enable);

// Go 包装器专用函数
void connectionCallbackWrapper(int fd, int connected, void *user_data);
void dataCallbackWrapper(int fd, const char *data, size_t length, void *user_data);
void errorCallbackWrapper(int fd, int error_code, const char *error_msg, void *user_data);
void statsCallbackWrapper(const epoll_server_stats_t *stats, void *user_data);

// 内部函数
static int set_nonblocking(int fd);
static int set_socket_options(int fd, const epoll_server_config_t *config);
static void* worker_thread(void *arg);
static void handle_new_connection(epoll_server_t *server);
static void handle_client_data(epoll_server_t *server, int fd);
static void handle_client_disconnect(epoll_server_t *server, int fd);
static void update_connection_stats(epoll_server_t *server, int fd, size_t bytes_sent, size_t bytes_received);
static void* heartbeat_thread_func(void *arg);
static void log_message(epoll_server_t *server, int level, const char *format, va_list args);

// 批量操作
int epoll_server_send_batch(epoll_server_t *server, const int *fds, int count, const char *data, size_t len);
int epoll_server_disconnect_batch(epoll_server_t *server, const int *fds, int count);

// 事件处理
typedef int (*event_handler_t)(epoll_server_t *server, int fd, uint32_t events);
int epoll_server_set_event_handler(epoll_server_t *server, event_handler_t handler);

// 安全功能
int epoll_server_enable_ssl(epoll_server_t *server, int enable);
int epoll_server_set_ssl_cert(epoll_server_t *server, const char *cert_path, const char *key_path);
int epoll_server_enable_client_auth(epoll_server_t *server, int enable);

// 插件系统
typedef struct {
    void *handle;
    const char *name;
    int (*init)(epoll_server_t *server);
    void (*cleanup)(epoll_server_t *server);
    int (*handle_data)(epoll_server_t *server, int fd, const char *data, size_t len);
} epoll_server_plugin_t;

int epoll_server_load_plugin(epoll_server_t *server, const char *plugin_path);
int epoll_server_unload_plugin(epoll_server_t *server, const char *plugin_name);
int epoll_server_enable_plugins(epoll_server_t *server, int enable);

#endif // EPOLL_SERVER_ENHANCED_H