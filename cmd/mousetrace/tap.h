#pragma once

#include <stdint.h>

// trace_event carries the CGEvent fields mousetrace records.
typedef struct {
    int type;
    uint64_t timestamp;
    double x, y;
    double delta_x, delta_y;
    int64_t button, clicks;
    double scroll_x, scroll_y, scroll_z;
} trace_event;

int trace_start(int keyboard);
int trace_step(void);
void trace_stop(void);
int trace_request_access(void);
