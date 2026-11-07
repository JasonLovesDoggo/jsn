#pragma once

#include <stdint.h>

// world_uptime_ns uses the same clock as CGEvent timestamps, which pauses
// during sleep. world_awake_ns keeps counting through sleep.
uint64_t world_uptime_ns(void);
uint64_t world_awake_ns(void);

void world_init(void);
int world_accessibility_trusted(int prompt);

// Each returns a malloc'd JSON object the caller frees, or NULL when the state
// is unavailable (usually because Accessibility is not granted).
char *world_focus(void);
char *world_hover(double x, double y);
char *world_displays(void);
char *world_screen(void);
char *world_page(int max_nodes);
char *world_tree(int max_nodes, double budget_seconds);
