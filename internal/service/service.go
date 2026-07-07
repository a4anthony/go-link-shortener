package service

import "time"

// touchTimeout bounds best-effort bookkeeping writes (e.g. api_keys.last_used_at)
// that run detached from the request lifecycle.
const touchTimeout = 3 * time.Second
