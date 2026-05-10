// atlas.hcl
//
// Atlas schema-diff 配置。GORM struct 是 schema 的 source of truth：
// `db/atlas` 程序按 ATLAS_DIALECT 把 GORM model 翻译成对应方言 DDL，
// atlas 用其与目标库对比，生成版本化 migration 文件到方言专属目录。
//
// 工作流：
//   make migrate-diff name=add_xxx        // 生成新 migration
//   make migrate-status                   // 查看待应用 migration
//   make migrate-apply                    // 应用 migration 到本地 DB
//
// 默认 migration 方言是 MySQL；运行时驱动由 data.db.user.driver 决定。
// PostgreSQL 使用 local_postgres env 与 db/migrations/postgres 目录。

data "external_schema" "gorm" {
  program = [
    "go",
    "run",
    "-mod=mod",
    "./db/atlas",
  ]
}

env "local_mysql" {
  src = data.external_schema.gorm.url
  // 本地 DB：与 deploy/docker-compose 中 user-db 容器对齐（端口 3380）
  url = "mysql://root:123456@127.0.0.1:3380/user"
  // dev DB：复用 user-db 容器内的独立 schema，避免 atlas 自启 docker 容器
  // （在 OrbStack 下 docker:// URL 会超时）。需提前 CREATE DATABASE atlas_dev。
  dev = "mysql://root:123456@127.0.0.1:3380/atlas_dev"
  migration {
    dir = "file://db/migrations/mysql"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

env "local_postgres" {
  src = data.external_schema.gorm.url
  // 需自行启动 PostgreSQL，并创建 user 与 atlas_dev 两个 database。
  url = "postgres://postgres:123456@127.0.0.1:5432/user?sslmode=disable"
  dev = "postgres://postgres:123456@127.0.0.1:5432/atlas_dev?sslmode=disable"
  migration {
    dir = "file://db/migrations/postgres"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}
