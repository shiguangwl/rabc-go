-- Prefix Casbin role subjects so role sid never shares the user-id namespace.
UPDATE "casbin_rule"
SET "v0" = 'role:' || "v0"
WHERE "ptype" = 'p'
  AND "v0" IS NOT NULL
  AND "v0" <> ''
  AND "v0" NOT LIKE 'role:%';

UPDATE "casbin_rule"
SET "v1" = 'role:' || "v1"
WHERE "ptype" = 'g'
  AND "v1" IS NOT NULL
  AND "v1" <> ''
  AND "v1" NOT LIKE 'role:%';

-- Permission resources are identified by menu.path and api(path, method).
-- Resource rows are hard-deleted by the application after this migration; remove
-- historical soft-deleted rows first so global unique keys do not stay occupied.
DELETE FROM "menu" WHERE "deleted_at" IS NOT NULL;
DELETE FROM "api" WHERE "deleted_at" IS NOT NULL;

-- 先把历史脏数据（NULL 或空串）改成 /_legacy_<id>。
UPDATE "menu" SET "path" = '/_legacy_' || "id" WHERE "path" IS NULL OR "path" = '';

-- 历史重复资源保留最小 id 的原 key，其余行改成可诊断的占位 key，避免
-- CREATE UNIQUE INDEX 在长期运行库上被脏数据阻断。
UPDATE "menu" AS m
SET "path" = '/_duplicate_menu_' || m."id"
FROM (
  SELECT "id"
  FROM (
    SELECT
      "id",
      MIN("id") OVER (PARTITION BY "path") AS keep_id,
      COUNT(*) OVER (PARTITION BY "path") AS duplicate_count
    FROM "menu"
  ) AS ranked
  WHERE duplicate_count > 1 AND "id" <> keep_id
) AS d
WHERE m."id" = d."id";

UPDATE "api" AS a
SET "path" = '/_duplicate_api_' || a."id"
FROM (
  SELECT "id"
  FROM (
    SELECT
      "id",
      MIN("id") OVER (PARTITION BY "path", "method") AS keep_id,
      COUNT(*) OVER (PARTITION BY "path", "method") AS duplicate_count
    FROM "api"
  ) AS ranked
  WHERE duplicate_count > 1 AND "id" <> keep_id
) AS d
WHERE a."id" = d."id";

ALTER TABLE "menu" ALTER COLUMN "path" SET NOT NULL;
COMMENT ON COLUMN "menu"."path" IS '前端路由路径';
CREATE UNIQUE INDEX IF NOT EXISTS "idx_menu_path" ON "menu" ("path");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_api_path_method" ON "api" ("path", "method");
