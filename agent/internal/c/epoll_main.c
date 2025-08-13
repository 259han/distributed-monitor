#include "epoll_server.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <signal.h>

static epoll_server_t *server = NULL;

// 数据回调函数
void data_callback(int fd, const char *data, int len, void *user_data) {
    (void)user_data; // 避免未使用参数警告
    printf("收到来自fd %d的数据: %.*s\n", fd, len, data);
    
    // 回显数据
    epoll_server_send_data(server, fd, data, len);
}

// 连接回调函数
void connection_callback(int fd, int connected, void *user_data) {
    (void)user_data; // 避免未使用参数警告
    if (connected) {
        printf("新连接: fd %d\n", fd);
    } else {
        printf("连接断开: fd %d\n", fd);
    }
}

// 信号处理函数
void signal_handler(int sig) {
    printf("接收到信号 %d，正在关闭服务器...\n", sig);
    if (server) {
        epoll_server_stop(server);
        epoll_server_destroy(server);
        server = NULL;
    }
    exit(0);
}

#ifdef COMPILE_STANDALONE
int main() {
    printf("启动epoll服务器...\n");
    
    // 设置信号处理
    signal(SIGINT, signal_handler);
    signal(SIGTERM, signal_handler);
    
    // 创建服务器
    server = epoll_server_create(8080);
    if (!server) {
        printf("创建服务器失败\n");
        return 1;
    }
    
    // 设置回调函数
    epoll_server_set_callback(server, data_callback, connection_callback, NULL);
    
    // 启动服务器
    if (epoll_server_start(server) != 0) {
        printf("启动服务器失败\n");
        epoll_server_destroy(server);
        return 1;
    }
    
    printf("epoll服务器已启动，监听端口 8080\n");
    printf("按Ctrl+C停止服务器\n");
    
    // 主循环
    while (1) {
        sleep(1);
    }
    
    return 0;
}
#endif