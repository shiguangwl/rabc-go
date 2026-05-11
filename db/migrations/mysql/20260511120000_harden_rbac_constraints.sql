-- Prefix Casbin role subjects so role sid never shares the user-id namespace.
UPDATE `casbin_rule`
SET `v0` = CONCAT('role:', `v0`)
WHERE `ptype` = 'p'
  AND `v0` IS NOT NULL
  AND `v0` <> ''
  AND `v0` NOT LIKE 'role:%';

UPDATE `casbin_rule`
SET `v1` = CONCAT('role:', `v1`)
WHERE `ptype` = 'g'
  AND `v1` IS NOT NULL
  AND `v1` <> ''
  AND `v1` NOT LIKE 'role:%';

-- Permission resources are identified by menu.path and api(path, method).
-- Resource rows are hard-deleted by the application after this migration; remove
-- historical soft-deleted rows first so global unique keys do not stay occupied.
DELETE FROM `menu` WHERE `deleted_at` IS NOT NULL;
DELETE FROM `api` WHERE `deleted_at` IS NOT NULL;

-- 先把历史脏数据（NULL 或空串）改成 /_legacy_<id>。
UPDATE `menu` SET `path` = CONCAT('/_legacy_', `id`) WHERE `path` IS NULL OR `path` = '';

-- 历史重复资源保留最小 id 的原 key，其余行改成可诊断的占位 key，避免
-- CREATE UNIQUE INDEX 在长期运行库上被脏数据阻断。
UPDATE `menu` AS m
JOIN (
  SELECT `id`
  FROM (
    SELECT m1.`id`
    FROM `menu` AS m1
    JOIN (
      SELECT `path`, MIN(`id`) AS keep_id
      FROM `menu`
      GROUP BY `path`
      HAVING COUNT(*) > 1
    ) AS d ON d.`path` = m1.`path`
    WHERE m1.`id` <> d.keep_id
  ) AS duplicate_menu
) AS d ON d.`id` = m.`id`
SET m.`path` = CONCAT('/_duplicate_menu_', m.`id`);

UPDATE `api` AS a
JOIN (
  SELECT `id`
  FROM (
    SELECT a1.`id`
    FROM `api` AS a1
    JOIN (
      SELECT `path`, `method`, MIN(`id`) AS keep_id
      FROM `api`
      GROUP BY `path`, `method`
      HAVING COUNT(*) > 1
    ) AS d ON d.`path` = a1.`path` AND d.`method` = a1.`method`
    WHERE a1.`id` <> d.keep_id
  ) AS duplicate_api
) AS d ON d.`id` = a.`id`
SET a.`path` = CONCAT('/_duplicate_api_', a.`id`);

ALTER TABLE `menu` MODIFY COLUMN `path` varchar(255) NOT NULL COMMENT '前端路由路径';
CREATE UNIQUE INDEX `idx_menu_path` ON `menu` (`path`);
CREATE UNIQUE INDEX `idx_api_path_method` ON `api` (`path`, `method`);
