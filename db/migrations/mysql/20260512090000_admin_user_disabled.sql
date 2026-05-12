-- 增加 admin_users.is_disabled 列，支持账号禁用而非删除。
--
-- 设计 Why：
--   - 删除走 GORM 软删（deleted_at 非空），不可恢复且对 casbin policy 解绑
--     需要专门处理；禁用是更轻量级的"暂时停用"语义。
--   - 默认 FALSE 不影响存量行为；Login 路径需在密码校验前判 IsDisabled 拒登。
--   - 改密 / 删除 / 禁用 都会触发 AuthService.RevokeAllUserSessions 立即吊销。

ALTER TABLE `admin_users`
  ADD COLUMN `is_disabled` BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否禁用';
