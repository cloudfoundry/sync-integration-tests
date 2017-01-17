package locket

import (
	"path"
	"time"
)

const DefaultSessionTTL = 15 * time.Second
const MonitorRetryTime = 2 * time.Second
const RetryInterval = 5 * time.Second

const LockSchemaRoot = "v1/locks"

func LockSchemaPath(lockName ...string) string {
	return path.Join(LockSchemaRoot, path.Join(lockName...))
}
