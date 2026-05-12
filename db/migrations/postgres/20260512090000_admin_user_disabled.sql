-- 增加 admin_users.is_disabled 列，支持账号禁用而非删除。
-- 与 mysql/20260512090000_admin_user_disabled.sql 对等，仅 SQL 方言差异。

ALTER TABLE "admin_users"
  ADD COLUMN "is_disabled" BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN "admin_users"."is_disabled" IS '是否禁用';
