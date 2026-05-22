-- Create "admin_users" table
CREATE TABLE `admin_users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `username` varchar(50) NOT NULL COMMENT "用户名",
  `nickname` varchar(50) NOT NULL COMMENT "昵称",
  `password` varchar(255) NOT NULL COMMENT "密码",
  `email` varchar(100) NOT NULL COMMENT "电子邮件",
  `phone` varchar(20) NOT NULL COMMENT "手机号",
  `is_disabled` bool NOT NULL DEFAULT 0 COMMENT "是否禁用",
  `last_login_at` datetime(3) NULL COMMENT "最后登录时间",
  PRIMARY KEY (`id`),
  INDEX `idx_admin_users_deleted_at` (`deleted_at`),
  UNIQUE INDEX `idx_admin_users_username` (`username`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "api" table
CREATE TABLE `api` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `group_name` varchar(100) NOT NULL COMMENT "API分组",
  `name` varchar(100) NOT NULL COMMENT "API名称",
  `path` varchar(255) NOT NULL COMMENT "API路径",
  `method` varchar(20) NOT NULL COMMENT "HTTP方法",
  PRIMARY KEY (`id`),
  INDEX `idx_api_deleted_at` (`deleted_at`),
  UNIQUE INDEX `idx_api_path_method` (`path`, `method`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "casbin_rule" table
CREATE TABLE `casbin_rule` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `ptype` varchar(100) NULL,
  `v0` varchar(100) NULL,
  `v1` varchar(100) NULL,
  `v2` varchar(100) NULL,
  `v3` varchar(100) NULL,
  `v4` varchar(100) NULL,
  `v5` varchar(100) NULL,
  PRIMARY KEY (`id`),
  UNIQUE INDEX `idx_casbin_rule` (`ptype`, `v0`, `v1`, `v2`, `v3`, `v4`, `v5`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "menu" table
CREATE TABLE `menu` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `parent_id` bigint unsigned NULL COMMENT "父级菜单id，0 表示根菜单",
  `path` varchar(255) NOT NULL COMMENT "前端路由路径",
  `title` varchar(100) NULL COMMENT "菜单显示标题",
  `name` varchar(100) NULL COMMENT "路由唯一标识，对应前端路由 name",
  `component` varchar(255) NULL COMMENT "绑定组件，常用：Iframe/RouteView/ComponentError",
  `locale` varchar(100) NULL COMMENT "i18n key",
  `icon` varchar(100) NULL COMMENT "图标",
  `redirect` varchar(255) NULL COMMENT "重定向地址",
  `url` varchar(255) NULL COMMENT "iframe 模式下的跳转 URL",
  `keep_alive` bool NULL DEFAULT 0 COMMENT "是否保活页面状态",
  `hide_in_menu` bool NULL DEFAULT 0 COMMENT "是否在菜单中隐藏",
  `target` varchar(20) NULL COMMENT "链接打开方式：_blank/_self/_parent",
  `weight` bigint NULL DEFAULT 0 COMMENT "排序权重，越大越靠前",
  PRIMARY KEY (`id`),
  INDEX `idx_menu_deleted_at` (`deleted_at`),
  INDEX `idx_menu_parent_id` (`parent_id`),
  UNIQUE INDEX `idx_menu_path` (`path`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "roles" table
CREATE TABLE `roles` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `name` varchar(100) NULL COMMENT "角色名",
  `sid` varchar(100) NULL COMMENT "角色标识",
  PRIMARY KEY (`id`),
  INDEX `idx_roles_deleted_at` (`deleted_at`),
  UNIQUE INDEX `idx_roles_name` (`name`),
  UNIQUE INDEX `idx_roles_sid` (`sid`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "sys_config" table
CREATE TABLE `sys_config` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `config_key` varchar(128) NOT NULL COMMENT "配置键，程序读取的稳定标识",
  `config_value` text NOT NULL COMMENT "配置值，统一以字符串存储",
  `value_type` varchar(16) NOT NULL DEFAULT "string" COMMENT "值类型：string/int/bool/json",
  `config_group` varchar(64) NOT NULL COMMENT "配置分组，决定前端 Tab",
  `title` varchar(128) NOT NULL COMMENT "展示名称",
  `remark` varchar(255) NOT NULL COMMENT "配置说明",
  `is_public` bool NOT NULL DEFAULT 0 COMMENT "是否允许未登录访问",
  `is_system` bool NOT NULL DEFAULT 0 COMMENT "内置配置，禁止删除与改元数据",
  `weight` bigint NOT NULL DEFAULT 0 COMMENT "组内排序权重，越大越靠前",
  PRIMARY KEY (`id`),
  INDEX `idx_sys_config_config_group` (`config_group`),
  UNIQUE INDEX `idx_sys_config_config_key` (`config_key`),
  INDEX `idx_sys_config_deleted_at` (`deleted_at`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
