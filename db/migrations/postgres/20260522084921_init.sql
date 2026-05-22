-- Create "admin_users" table
CREATE TABLE "public"."admin_users" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "username" character varying(50) NOT NULL,
  "nickname" character varying(50) NOT NULL,
  "password" character varying(255) NOT NULL,
  "email" character varying(100) NOT NULL,
  "phone" character varying(20) NOT NULL,
  "is_disabled" boolean NOT NULL DEFAULT false,
  "last_login_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_admin_users_deleted_at" to table: "admin_users"
CREATE INDEX "idx_admin_users_deleted_at" ON "public"."admin_users" ("deleted_at");
-- Create index "idx_admin_users_username" to table: "admin_users"
CREATE UNIQUE INDEX "idx_admin_users_username" ON "public"."admin_users" ("username");
-- Set comment to column: "username" on table: "admin_users"
COMMENT ON COLUMN "public"."admin_users"."username" IS '用户名';
-- Set comment to column: "nickname" on table: "admin_users"
COMMENT ON COLUMN "public"."admin_users"."nickname" IS '昵称';
-- Set comment to column: "password" on table: "admin_users"
COMMENT ON COLUMN "public"."admin_users"."password" IS '密码';
-- Set comment to column: "email" on table: "admin_users"
COMMENT ON COLUMN "public"."admin_users"."email" IS '电子邮件';
-- Set comment to column: "phone" on table: "admin_users"
COMMENT ON COLUMN "public"."admin_users"."phone" IS '手机号';
-- Set comment to column: "is_disabled" on table: "admin_users"
COMMENT ON COLUMN "public"."admin_users"."is_disabled" IS '是否禁用';
-- Set comment to column: "last_login_at" on table: "admin_users"
COMMENT ON COLUMN "public"."admin_users"."last_login_at" IS '最后登录时间';
-- Create "api" table
CREATE TABLE "public"."api" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "group_name" character varying(100) NOT NULL,
  "name" character varying(100) NOT NULL,
  "path" character varying(255) NOT NULL,
  "method" character varying(20) NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_api_deleted_at" to table: "api"
CREATE INDEX "idx_api_deleted_at" ON "public"."api" ("deleted_at");
-- Create index "idx_api_path_method" to table: "api"
CREATE UNIQUE INDEX "idx_api_path_method" ON "public"."api" ("path", "method");
-- Set comment to column: "group_name" on table: "api"
COMMENT ON COLUMN "public"."api"."group_name" IS 'API分组';
-- Set comment to column: "name" on table: "api"
COMMENT ON COLUMN "public"."api"."name" IS 'API名称';
-- Set comment to column: "path" on table: "api"
COMMENT ON COLUMN "public"."api"."path" IS 'API路径';
-- Set comment to column: "method" on table: "api"
COMMENT ON COLUMN "public"."api"."method" IS 'HTTP方法';
-- Create "casbin_rule" table
CREATE TABLE "public"."casbin_rule" (
  "id" bigserial NOT NULL,
  "ptype" character varying(100) NULL,
  "v0" character varying(100) NULL,
  "v1" character varying(100) NULL,
  "v2" character varying(100) NULL,
  "v3" character varying(100) NULL,
  "v4" character varying(100) NULL,
  "v5" character varying(100) NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_casbin_rule" to table: "casbin_rule"
CREATE UNIQUE INDEX "idx_casbin_rule" ON "public"."casbin_rule" ("ptype", "v0", "v1", "v2", "v3", "v4", "v5");
-- Create "menu" table
CREATE TABLE "public"."menu" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "parent_id" bigint NULL,
  "path" character varying(255) NOT NULL,
  "title" character varying(100) NULL,
  "name" character varying(100) NULL,
  "component" character varying(255) NULL,
  "locale" character varying(100) NULL,
  "icon" character varying(100) NULL,
  "redirect" character varying(255) NULL,
  "url" character varying(255) NULL,
  "keep_alive" boolean NULL DEFAULT false,
  "hide_in_menu" boolean NULL DEFAULT false,
  "target" character varying(20) NULL,
  "weight" bigint NULL DEFAULT 0,
  PRIMARY KEY ("id")
);
-- Create index "idx_menu_deleted_at" to table: "menu"
CREATE INDEX "idx_menu_deleted_at" ON "public"."menu" ("deleted_at");
-- Create index "idx_menu_parent_id" to table: "menu"
CREATE INDEX "idx_menu_parent_id" ON "public"."menu" ("parent_id");
-- Create index "idx_menu_path" to table: "menu"
CREATE UNIQUE INDEX "idx_menu_path" ON "public"."menu" ("path");
-- Set comment to column: "parent_id" on table: "menu"
COMMENT ON COLUMN "public"."menu"."parent_id" IS '父级菜单id，0 表示根菜单';
-- Set comment to column: "path" on table: "menu"
COMMENT ON COLUMN "public"."menu"."path" IS '前端路由路径';
-- Set comment to column: "title" on table: "menu"
COMMENT ON COLUMN "public"."menu"."title" IS '菜单显示标题';
-- Set comment to column: "name" on table: "menu"
COMMENT ON COLUMN "public"."menu"."name" IS '路由唯一标识，对应前端路由 name';
-- Set comment to column: "component" on table: "menu"
COMMENT ON COLUMN "public"."menu"."component" IS '绑定组件，常用：Iframe/RouteView/ComponentError';
-- Set comment to column: "locale" on table: "menu"
COMMENT ON COLUMN "public"."menu"."locale" IS 'i18n key';
-- Set comment to column: "icon" on table: "menu"
COMMENT ON COLUMN "public"."menu"."icon" IS '图标';
-- Set comment to column: "redirect" on table: "menu"
COMMENT ON COLUMN "public"."menu"."redirect" IS '重定向地址';
-- Set comment to column: "url" on table: "menu"
COMMENT ON COLUMN "public"."menu"."url" IS 'iframe 模式下的跳转 URL';
-- Set comment to column: "keep_alive" on table: "menu"
COMMENT ON COLUMN "public"."menu"."keep_alive" IS '是否保活页面状态';
-- Set comment to column: "hide_in_menu" on table: "menu"
COMMENT ON COLUMN "public"."menu"."hide_in_menu" IS '是否在菜单中隐藏';
-- Set comment to column: "target" on table: "menu"
COMMENT ON COLUMN "public"."menu"."target" IS '链接打开方式：_blank/_self/_parent';
-- Set comment to column: "weight" on table: "menu"
COMMENT ON COLUMN "public"."menu"."weight" IS '排序权重，越大越靠前';
-- Create "roles" table
CREATE TABLE "public"."roles" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "name" character varying(100) NULL,
  "sid" character varying(100) NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_roles_deleted_at" to table: "roles"
CREATE INDEX "idx_roles_deleted_at" ON "public"."roles" ("deleted_at");
-- Create index "idx_roles_name" to table: "roles"
CREATE UNIQUE INDEX "idx_roles_name" ON "public"."roles" ("name");
-- Create index "idx_roles_sid" to table: "roles"
CREATE UNIQUE INDEX "idx_roles_sid" ON "public"."roles" ("sid");
-- Set comment to column: "name" on table: "roles"
COMMENT ON COLUMN "public"."roles"."name" IS '角色名';
-- Set comment to column: "sid" on table: "roles"
COMMENT ON COLUMN "public"."roles"."sid" IS '角色标识';
-- Create "sys_config" table
CREATE TABLE "public"."sys_config" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "config_key" character varying(128) NOT NULL,
  "config_value" text NOT NULL,
  "value_type" character varying(16) NOT NULL DEFAULT 'string',
  "config_group" character varying(64) NOT NULL,
  "title" character varying(128) NOT NULL,
  "remark" character varying(255) NOT NULL,
  "is_public" boolean NOT NULL DEFAULT false,
  "is_system" boolean NOT NULL DEFAULT false,
  "weight" bigint NOT NULL DEFAULT 0,
  PRIMARY KEY ("id")
);
-- Create index "idx_sys_config_config_group" to table: "sys_config"
CREATE INDEX "idx_sys_config_config_group" ON "public"."sys_config" ("config_group");
-- Create index "idx_sys_config_config_key" to table: "sys_config"
CREATE UNIQUE INDEX "idx_sys_config_config_key" ON "public"."sys_config" ("config_key");
-- Create index "idx_sys_config_deleted_at" to table: "sys_config"
CREATE INDEX "idx_sys_config_deleted_at" ON "public"."sys_config" ("deleted_at");
-- Set comment to column: "config_key" on table: "sys_config"
COMMENT ON COLUMN "public"."sys_config"."config_key" IS '配置键，程序读取的稳定标识';
-- Set comment to column: "config_value" on table: "sys_config"
COMMENT ON COLUMN "public"."sys_config"."config_value" IS '配置值，统一以字符串存储';
-- Set comment to column: "value_type" on table: "sys_config"
COMMENT ON COLUMN "public"."sys_config"."value_type" IS '值类型：string/int/bool/json';
-- Set comment to column: "config_group" on table: "sys_config"
COMMENT ON COLUMN "public"."sys_config"."config_group" IS '配置分组，决定前端 Tab';
-- Set comment to column: "title" on table: "sys_config"
COMMENT ON COLUMN "public"."sys_config"."title" IS '展示名称';
-- Set comment to column: "remark" on table: "sys_config"
COMMENT ON COLUMN "public"."sys_config"."remark" IS '配置说明';
-- Set comment to column: "is_public" on table: "sys_config"
COMMENT ON COLUMN "public"."sys_config"."is_public" IS '是否允许未登录访问';
-- Set comment to column: "is_system" on table: "sys_config"
COMMENT ON COLUMN "public"."sys_config"."is_system" IS '内置配置，禁止删除与改元数据';
-- Set comment to column: "weight" on table: "sys_config"
COMMENT ON COLUMN "public"."sys_config"."weight" IS '组内排序权重，越大越靠前';
