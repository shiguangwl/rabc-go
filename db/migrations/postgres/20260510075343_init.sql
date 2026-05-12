CREATE TABLE "admin_users" (
  "id" bigserial,
  "created_at" timestamptz,
  "updated_at" timestamptz,
  "deleted_at" timestamptz,
  "username" varchar(50) NOT NULL,
  "nickname" varchar(50) NOT NULL,
  "password" varchar(255) NOT NULL,
  "email" varchar(100) NOT NULL,
  "phone" varchar(20) NOT NULL,
  "is_disabled" BOOLEAN NOT NULL DEFAULT FALSE,
  "last_login_at" TIMESTAMP NULL,
  PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX IF NOT EXISTS "idx_admin_users_username" ON "admin_users" ("username");
CREATE INDEX IF NOT EXISTS "idx_admin_users_deleted_at" ON "admin_users" ("deleted_at");
COMMENT ON COLUMN "admin_users"."username" IS '用户名';
COMMENT ON COLUMN "admin_users"."nickname" IS '昵称';
COMMENT ON COLUMN "admin_users"."password" IS '密码';
COMMENT ON COLUMN "admin_users"."email" IS '电子邮件';
COMMENT ON COLUMN "admin_users"."phone" IS '手机号';
COMMENT ON COLUMN "admin_users"."is_disabled" IS '是否禁用';
COMMENT ON COLUMN "admin_users"."last_login_at" IS '最后登录时间';

CREATE TABLE "menu" (
  "id" bigserial,
  "created_at" timestamptz,
  "updated_at" timestamptz,
  "deleted_at" timestamptz,
  "parent_id" bigint,
  "path" varchar(255) NOT NULL,
  "title" varchar(100),
  "name" varchar(100),
  "component" varchar(255),
  "locale" varchar(100),
  "icon" varchar(100),
  "redirect" varchar(255),
  "url" varchar(255),
  "keep_alive" boolean DEFAULT false,
  "hide_in_menu" boolean DEFAULT false,
  "target" varchar(20),
  "weight" bigint DEFAULT 0,
  PRIMARY KEY ("id")
);
CREATE INDEX IF NOT EXISTS "idx_menu_parent_id" ON "menu" ("parent_id");
CREATE INDEX IF NOT EXISTS "idx_menu_deleted_at" ON "menu" ("deleted_at");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_menu_path" ON "menu" ("path");
COMMENT ON COLUMN "menu"."parent_id" IS '父级菜单的id，使用整数表示';
COMMENT ON COLUMN "menu"."path" IS '前端路由路径';
COMMENT ON COLUMN "menu"."title" IS '标题，使用字符串表示';
COMMENT ON COLUMN "menu"."name" IS '同路由中的name，用于保活';
COMMENT ON COLUMN "menu"."component" IS '绑定的组件';
COMMENT ON COLUMN "menu"."locale" IS '本地化标识';
COMMENT ON COLUMN "menu"."icon" IS '图标，使用字符串表示';
COMMENT ON COLUMN "menu"."redirect" IS '重定向地址';
COMMENT ON COLUMN "menu"."url" IS 'iframe模式下的跳转url';
COMMENT ON COLUMN "menu"."keep_alive" IS '是否保活';
COMMENT ON COLUMN "menu"."hide_in_menu" IS '是否保活';
COMMENT ON COLUMN "menu"."target" IS '全连接跳转模式';
COMMENT ON COLUMN "menu"."weight" IS '排序权重';

CREATE TABLE "roles" (
  "id" bigserial,
  "created_at" timestamptz,
  "updated_at" timestamptz,
  "deleted_at" timestamptz,
  "name" varchar(100),
  "sid" varchar(100),
  PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX IF NOT EXISTS "idx_roles_sid" ON "roles" ("sid");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_roles_name" ON "roles" ("name");
CREATE INDEX IF NOT EXISTS "idx_roles_deleted_at" ON "roles" ("deleted_at");
COMMENT ON COLUMN "roles"."name" IS '角色名';
COMMENT ON COLUMN "roles"."sid" IS '角色标识';

CREATE TABLE "api" (
  "id" bigserial,
  "created_at" timestamptz,
  "updated_at" timestamptz,
  "deleted_at" timestamptz,
  "group_name" varchar(100) NOT NULL,
  "name" varchar(100) NOT NULL,
  "path" varchar(255) NOT NULL,
  "method" varchar(20) NOT NULL,
  PRIMARY KEY ("id")
);
CREATE INDEX IF NOT EXISTS "idx_api_deleted_at" ON "api" ("deleted_at");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_api_path_method" ON "api" ("path", "method");
COMMENT ON COLUMN "api"."group_name" IS 'API分组';
COMMENT ON COLUMN "api"."name" IS 'API名称';
COMMENT ON COLUMN "api"."path" IS 'API路径';
COMMENT ON COLUMN "api"."method" IS 'HTTP方法';

CREATE TABLE "casbin_rule" (
  "id" bigserial,
  "ptype" varchar(100),
  "v0" varchar(100),
  "v1" varchar(100),
  "v2" varchar(100),
  "v3" varchar(100),
  "v4" varchar(100),
  "v5" varchar(100),
  PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX IF NOT EXISTS "idx_casbin_rule" ON "casbin_rule" ("ptype", "v0", "v1", "v2", "v3", "v4", "v5");
