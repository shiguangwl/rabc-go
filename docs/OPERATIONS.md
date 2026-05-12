# 运维 SOP（OPERATIONS）

## 1. JWT key 轮换

**触发**：密钥泄露 / 例行 90 天轮换。

```bash
# 1. 生成新 key（32+ bytes）
openssl rand -base64 32

# 2. 编辑 config/prod.yml security.jwt.key 或下发新 env：
export APP_SECURITY_JWT_KEY="<new-base64-key>"

# 3. 灰度滚动 deploy（旧 access 仍按旧 key 验证至自然过期；refresh 走 Redis 不受影响）
# 4. 30min 后（access_ttl 已过期完毕）即可视为切换完成
# 5. 强制全员重登（可选，加速）：
#    redis-cli --scan --pattern 'auth:refresh:*' | xargs redis-cli DEL
#    redis-cli --scan --pattern 'auth:user:*:sessions' | xargs redis-cli DEL
```

注意：旧 JWT key 进过 git 历史等同公开，必须用 `git filter-repo` 清理。

## 2. 紧急踢全员

**触发**：管理员账号疑似被入侵 / 大规模权限误配置。

```bash
# 踢单个 uid（推荐，影响面小）
# 通过管理端 UI 操作即可：用户管理 → [会话] → 全部踢出

# 踢全员（影响所有用户）
redis-cli --scan --pattern 'auth:refresh:*' | xargs -r redis-cli DEL
redis-cli --scan --pattern 'auth:refresh:tomb:*' | xargs -r redis-cli DEL
redis-cli --scan --pattern 'auth:user:*:sessions' | xargs -r redis-cli DEL

# 同时建议轮换 JWT key（见 §1），让既有 access 立即失效
```

## 3. Redis 故障恢复

**现象**：refresh/login 接口 500；zap 日志含 `connection refused`。

```bash
# 1. 检查 Redis 进程与 AOF 文件
redis-cli ping
ls -lh /var/lib/redis/appendonly.aof

# 2. 重启 Redis（AOF everysec 模式最多丢 1s 数据）
systemctl restart redis-server

# 3. 验证关键 key 是否回来
redis-cli --scan --pattern 'auth:refresh:*' | head

# 4. 启动期：用户的 access 在 sessionStorage 仍可用（自验签）；
#    Redis 恢复后，next refresh 自然走 Lua 轮换继续。
#    若 refresh:{sid} 已丢失 → 该用户 refresh 返 401 → 前端跳 /login
```

## 4. 数据备份与还原

**Redis**（refresh / sessions / tomb）：

```bash
# 每日凌晨 RDB 快照 + 拷贝 AOF
redis-cli BGSAVE
cp /var/lib/redis/dump.rdb       /backup/redis/$(date +%F).rdb
cp /var/lib/redis/appendonly.aof /backup/redis/$(date +%F).aof
```

**MySQL**（admin_users / roles / casbin_rule）：

```bash
# 每日凌晨 mysqldump
mysqldump -u root -p \
  --single-transaction --routines --triggers \
  rabc_go > /backup/mysql/$(date +%F).sql

# 还原
mysql -u root -p rabc_go < /backup/mysql/2026-05-12.sql
```

注意：还原后必须重启服务 → 触发 Casbin LoadPolicy（rbac 内存视图重建）。

---

## 端到端冒烟脚本

```bash
# 启动服务
make run-local &
sleep 3

# 登录
LOGIN=$(curl -s -X POST http://127.0.0.1:8000/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<seed-password>"}')
AT=$(echo "$LOGIN" | jq -r .data.accessToken)
RT=$(echo "$LOGIN" | jq -r .data.refreshToken)
EXP=$(echo "$LOGIN" | jq -r .data.expiresIn)
[ ${#RT} -ge 40 ] && echo "RT len OK ($(echo -n $RT | wc -c))" || echo "RT FAIL"
[ "$EXP" -gt 0 ] && echo "expiresIn OK ($EXP)" || echo "expiresIn FAIL"

# refresh：拿新对凭证
REFRESH=$(curl -s -X POST http://127.0.0.1:8000/v1/auth/refresh \
  -H 'Content-Type: application/json' \
  -d "{\"refreshToken\":\"$RT\"}")
NEW_AT=$(echo "$REFRESH" | jq -r .data.accessToken)
NEW_RT=$(echo "$REFRESH" | jq -r .data.refreshToken)
[ -n "$NEW_AT" ] && echo "refresh OK" || echo "refresh FAIL"

# 用旧 RT 再 refresh → 期望 1005 ErrRefreshReused + 该 user 全部 session 清空
OLD_REUSE=$(curl -s -X POST http://127.0.0.1:8000/v1/auth/refresh \
  -H 'Content-Type: application/json' \
  -d "{\"refreshToken\":\"$RT\"}")
echo "reuse_detected → $OLD_REUSE"

# 验证连坐：sessions 已清空
UID_BY=$(redis-cli --scan --pattern 'auth:user:*:sessions' | head -1)
redis-cli ZCARD "$UID_BY"   # 期望 0

# 验证墓碑存在
redis-cli --scan --pattern 'auth:refresh:tomb:*' | head -1
```
