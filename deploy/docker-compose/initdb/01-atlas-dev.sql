-- atlas dev 库：用于 atlas migrate diff 计算 schema 差异，与业务库 user 隔离。
-- 复用同一个 MySQL 容器以避免在 OrbStack 上 atlas docker:// 启动超时；
-- 独立 schema 名能保证 atlas 的 DROP/CREATE 行为不会误及业务库。
CREATE DATABASE IF NOT EXISTS `atlas_dev` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
