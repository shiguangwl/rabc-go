-- Create "admin_users" table
CREATE TABLE `admin_users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `username` varchar(50) NOT NULL COMMENT '用户名',
  `nickname` varchar(50) NOT NULL COMMENT '昵称',
  `password` varchar(255) NOT NULL COMMENT '密码',
  `email` varchar(100) NOT NULL COMMENT '电子邮件',
  `phone` varchar(20) NOT NULL COMMENT '手机号',
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
  `group_name` varchar(100) NOT NULL COMMENT 'API分组',
  `name` varchar(100) NOT NULL COMMENT 'API名称',
  `path` varchar(255) NOT NULL COMMENT 'API路径',
  `method` varchar(20) NOT NULL COMMENT 'HTTP方法',
  PRIMARY KEY (`id`),
  INDEX `idx_api_deleted_at` (`deleted_at`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "menu" table
CREATE TABLE `menu` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `parent_id` bigint unsigned NULL COMMENT "父级菜单的id，使用整数表示",
  `path` varchar(255) NULL COMMENT "地址",
  `title` varchar(100) NULL COMMENT "标题，使用字符串表示",
  `name` varchar(100) NULL COMMENT "同路由中的name，用于保活",
  `component` varchar(255) NULL COMMENT "绑定的组件",
  `locale` varchar(100) NULL COMMENT "本地化标识",
  `icon` varchar(100) NULL COMMENT "图标，使用字符串表示",
  `redirect` varchar(255) NULL COMMENT "重定向地址",
  `url` varchar(255) NULL COMMENT "iframe模式下的跳转url",
  `keep_alive` bool NULL DEFAULT 0 COMMENT "是否保活",
  `hide_in_menu` bool NULL DEFAULT 0 COMMENT "是否保活",
  `target` varchar(20) NULL COMMENT "全连接跳转模式",
  `weight` bigint NULL DEFAULT 0 COMMENT "排序权重",
  PRIMARY KEY (`id`),
  INDEX `idx_menu_deleted_at` (`deleted_at`),
  INDEX `idx_menu_parent_id` (`parent_id`)
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
