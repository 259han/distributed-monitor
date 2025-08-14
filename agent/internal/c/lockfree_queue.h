#ifndef LOCKFREE_QUEUE_H
#define LOCKFREE_QUEUE_H

#include <stdlib.h>
#include <stdatomic.h>

// Maximum number of connections the queue can hold
#define LFQ_MAX_CONNECTIONS 4096

// Structure to hold connection data
typedef struct {
    int fd;                      // File descriptor of the connection
    void* data;                  // Additional connection data
    atomic_int state;            // Connection state (0: inactive, 1: active, 2: closing)
} lfq_connection_t;

// Lock-free queue structure
typedef struct {
    lfq_connection_t connections[LFQ_MAX_CONNECTIONS]; // Array of connections
    atomic_size_t head;          // Head index of the queue
    atomic_size_t tail;          // Tail index of the queue
    size_t capacity;             // Maximum capacity of the queue
    atomic_int initialized;      // Flag to indicate if queue is initialized
} lockfree_queue_t;

// Initialize the lock-free queue
int lfq_init(lockfree_queue_t* queue, size_t capacity);

// Destroy the lock-free queue
void lfq_destroy(lockfree_queue_t* queue);

// Enqueue a new connection
int lfq_enqueue(lockfree_queue_t* queue, int fd, void* data);

// Dequeue a connection
int lfq_dequeue(lockfree_queue_t* queue, lfq_connection_t* conn);

// Check if queue is empty
int lfq_is_empty(lockfree_queue_t* queue);

// Check if queue is full
int lfq_is_full(lockfree_queue_t* queue);

// Get current size of the queue
size_t lfq_size(lockfree_queue_t* queue);

#endif // LOCKFREE_QUEUE_H
