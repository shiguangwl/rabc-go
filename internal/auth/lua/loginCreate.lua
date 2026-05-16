-- loginCreate.lua: 登录时原子创建 Refresh Token 会话。
--
-- 登录路径需要"SET refresh_key + ZADD sessions"两步原子化，否则与并发
-- RevokeAllUserSessions（改密 / 禁用 / 删除 / 管理员踢下线）有 race window：
--     T0: Tab1 login.lua 之前      T1: admin RevokeAll → ZRANGE 不含 Tab1 sid
--     T2: Tab1 SET refresh_key     T3: Tab1 ZADD sessions（漏被 DEL）
--     → Tab1 RT 在 Redis 实际存在但 sessions 索引不一致，泄漏一个 7d 会话。
-- Redis 单线程执行脚本，能保证这两步对外原子可见。
--
-- KEYS:
--   KEYS[1] = auth:refresh:{sid}
--   KEYS[2] = auth:user:{uid}:sessions
-- ARGV:
--   ARGV[1] = record_json
--   ARGV[2] = refresh_ttl_sec
--   ARGV[3] = sid
--   ARGV[4] = exp_ts

redis.call('SET',  KEYS[1], ARGV[1], 'EX', tonumber(ARGV[2]))
redis.call('ZADD', KEYS[2], tonumber(ARGV[4]), ARGV[3])
return { 0 }
