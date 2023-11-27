#!/usr/bin/env bash
exec ../com_github_redis_redis/redis_cli -s "$TMPDIR/redis.sock" PING