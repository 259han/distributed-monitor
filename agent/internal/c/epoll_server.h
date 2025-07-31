#ifndef EPOLL_SERVER_H
#define EPOLL_SERVER_H

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
#include <sys/time.h>
#include <sys/resource.h>
#include <signal.h>
#include <time.h>

// 服务器配置
#define MAX_EVENTS 1024
#define MAX_CONNECTIONS 10000
#define BUFFER_SIZE 8192
#define DEFAULT_PORT 8080
#define BACKLOG 1024
#define MAX_THREADS 8
#define HEARTBEAT_INTERVAL 30

// 错误码定义
#define EPOLL_SERVER_SUCCESS 0
#define EPOLL_SERVER_ERROR -1
#define EPOLL_SERVER_INVALID_PARAM -2
#define EPOLL_SERVER_MEMORY_ERROR -3
#define EPOLL_SERVER_SOCKET_ERROR -4
#define EPOLL_SERVER_BIND_ERROR -5
#define EPOLL_SERVER_LISTEN_ERROR -6
#define EPOLL_SERVER_EPOLL_ERROR -7
#define EPOLL_SERVER_THREAD_ERROR -8

// 连接状态
#define CONNECTION_STATE_CONNECTED 1
#define CONNECTION_STATE_DISCONNECTED 0
#define CONNECTION_STATE_ERROR -1

// 统计信息结构
typedef struct {
    unsigned long total_connections;
    unsigned long active_connections;
    unsigned long total_bytes_sent;
    unsigned long total_bytes_received;
    unsigned long total_messages;
    time_t start_time;
    double avg_response_time;
} server_stats_t;

// 回调函数类型（前向声明）
typedef void (*data_callback_t)(int fd, const char *data, int len, void *user_data);
typedef void (*connection_callback_t)(int fd, int connected, void *user_data);
typedef void (*error_callback_t)(int fd, int error_code, const char *error_msg, void *user_data);
typedef void (*stats_callback_t)(const server_stats_t *stats, void *user_data);

// 连接信息结构
typedef struct {
    int fd;
    struct sockaddr_in addr;
    char buffer[BUFFER_SIZE];
    int buffer_len;
    int state;
    time_t connect_time;
    time_t last_activity;
    unsigned long bytes_sent;
    unsigned long bytes_received;
    unsigned long messages_sent;
    unsigned long messages_received;
    char client_ip[INET_ADDRSTRLEN];
    int client_port;
} connection_t;

// 服务器结构
typedef struct {
    int epoll_fd;
    int server_fd;
    connection_t *connections;
    int connection_count;
    int max_connections;
    pthread_t *thread_pool;
    int thread_count;
    int running;
    int port;
    
    // 回调函数
    data_callback_t data_callback;
    connection_callback_t connection_callback;
    error_callback_t error_callback;
    stats_callback_t stats_callback;
    void *user_data;
    
    // 统计信息
    server_stats_t stats;
    pthread_mutex_t stats_mutex;
    
    // 配置选项
    int enable_keepalive;
    int keepalive_time;
    int keepalive_interval;
    int keepalive_probes;
    int enable_nagle;
    int socket_timeout;
    int max_buffer_size;
    
    // 心跳检测
    pthread_t heartbeat_thread;
    int heartbeat_enabled;
} epoll_server_t;

// 事件类型
typedef enum {
    EPOLL_EVENT_READ = 1,
    EPOLL_EVENT_WRITE = 2,
    EPOLL_EVENT_ERROR = 4,
    EPOLL_EVENT_HUP = 8,
    EPOLL_EVENT_RDHUP = 16
} epoll_event_type_t;

// 消息结构
typedef struct {
    int fd;
    char *data;
    int len;
    struct sockaddr_in addr;
} message_t;

// 函数声明
epoll_server_t* epoll_server_create(int port);
epoll_server_t* epoll_server_create_with_config(int port, int max_connections, int thread_count);
int epoll_server_start(epoll_server_t *server);
void epoll_server_stop(epoll_server_t *server);
void epoll_server_destroy(epoll_server_t *server);

// 回调函数设置
int epoll_server_set_callback(epoll_server_t *server, data_callback_t data_cb, connection_callback_t conn_cb, void *user_data);
int epoll_server_set_error_callback(epoll_server_t *server, error_callback_t error_cb);
int epoll_server_set_stats_callback(epoll_server_t *server, stats_callback_t stats_cb);

// 数据发送
int epoll_server_send_data(epoll_server_t *server, int fd, const char *data, int len);
int epoll_server_send_data_to_all(epoll_server_t *server, const char *data, int len);
int epoll_server_send_data_except(epoll_server_t *server, int except_fd, const char *data, int len);

// 连接管理
int epoll_server_close_connection(epoll_server_t *server, int fd);
int epoll_server_get_connection_info(epoll_server_t *server, int fd, connection_t *info);
int epoll_server_is_connection_active(epoll_server_t *server, int fd);

// 配置设置
int epoll_server_set_keepalive(epoll_server_t *server, int enable, int time, int interval, int probes);
int epoll_server_set_nagle(epoll_server_t *server, int enable);
int epoll_server_set_timeout(epoll_server_t *server, int timeout);
int epoll_server_set_buffer_size(epoll_server_t *server, int size);

// 统计信息
int epoll_server_get_stats(epoll_server_t *server, server_stats_t *stats);
int epoll_server_reset_stats(epoll_server_t *server);
void epoll_server_print_stats(epoll_server_t *server);

// 心跳检测
int epoll_server_enable_heartbeat(epoll_server_t *server, int enable);
int epoll_server_check_inactive_connections(epoll_server_t *server, int timeout_seconds);

// 工具函数
const char* epoll_server_strerror(int error_code);
int epoll_server_get_fd_count(epoll_server_t *server);
int epoll_server_broadcast_message(epoll_server_t *server, const message_t *msg);

// 信号处理
void epoll_server_setup_signal_handlers(epoll_server_t *server);
void epoll_server_handle_signal(int sig);

// 内存管理
int epoll_server_resize_connections(epoll_server_t *server, int new_size);
int epoll_server_cleanup_inactive_connections(epoll_server_t *server);

// 线程池管理
int epoll_server_set_thread_count(epoll_server_t *server, int count);
int epoll_server_get_thread_count(epoll_server_t *server);

// 日志记录
void epoll_server_set_log_level(int level);
void epoll_server_log(const char *format, ...);

#endif // EPOLL_SERVER_H