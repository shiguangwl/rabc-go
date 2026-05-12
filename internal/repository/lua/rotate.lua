-- rotate.lua: Refresh Token 原子轮换脚本。
--
-- Refresh Token 必须严格一次性使用；校验旧 RT、写入新 RT、删除旧 RT 与写墓碑
-- 必须在 Redis 单线程内原子完成。
--
-- 错误码：
--   0 = success     正常轮换
--   1 = expired     refresh_key 不存在，墓碑也不存在（自然过期）
--   2 = reused      hash 不匹配 / 墓碑命中 / JSON 损坏
--
-- KEYS:
--   KEYS[1] = auth:refresh:{old_sid}
--   KEYS[2] = auth:refresh:tomb:{old_sid}
--   KEYS[3] = auth:refresh:{new_sid}
--   KEYS[4] = auth:user:{uid}:sessions
-- ARGV:
--   ARGV[1] = expected_hash(old_rt_raw)
--   ARGV[2] = new_record_json
--   ARGV[3] = refresh_ttl_sec
--   ARGV[4] = rotation_tomb_ttl_sec
--   ARGV[5] = new_sid
--   ARGV[6] = new_exp_ts
--   ARGV[7] = old_sid
--   ARGV[8] = uid_str   -- 墓碑 value 必须能定位需要吊销的用户。

local rec = redis.call('GET', KEYS[1])
if not rec then
  local tomb = redis.call('GET', KEYS[2])
  if tomb then return { 2 } end  -- 墓碑命中，归 reused
  return { 1 }                    -- 自然过期
end

-- pcall 隔离 JSON 损坏：解析失败或字段缺失 → 归 reused（不再细分 data_corruption）
local ok, parsed = pcall(cjson.decode, rec)
if not ok or type(parsed) ~= 'table' or not parsed.th then
  return { 2 }
end
if parsed.th ~= ARGV[1] then return { 2 } end  -- hash 不匹配按复用处理

-- 正常轮换：新 RT 写入 + sessions 索引更新 + 旧 RT 入墓碑
redis.call('SET',  KEYS[3], ARGV[2], 'EX', tonumber(ARGV[3]))
redis.call('ZADD', KEYS[4], tonumber(ARGV[6]), ARGV[5])
redis.call('DEL',  KEYS[1])
redis.call('ZREM', KEYS[4], ARGV[7])
redis.call('SET',  KEYS[2], ARGV[8], 'EX', tonumber(ARGV[4]))
return { 0 }
