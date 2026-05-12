-- 增加 admin_users.last_login_at 列，记录最近一次成功登录时间。

ALTER TABLE "admin_users"
  ADD COLUMN "last_login_at" TIMESTAMP NULL;

COMMENT ON COLUMN "admin_users"."last_login_at" IS '最后登录时间';
