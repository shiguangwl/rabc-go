-- 增加 admin_users.last_login_at 列，记录最近一次成功登录时间。
--
-- 设计 Why：
--   - 用 NULL 表示"从未登录过"，与 0000-00-00 / epoch 时间戳相比语义清晰；
--   - 仅在 AuthService.Login 校验通过后异步更新，对登录性能近似无影响；
--   - GetAdminUser / GetAdminUsers 响应返回该字段，便于管理端审计活跃度。

ALTER TABLE `admin_users`
  ADD COLUMN `last_login_at` DATETIME NULL COMMENT '最后登录时间';
