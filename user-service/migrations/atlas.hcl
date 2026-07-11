data "external_schema" "ent" {
  program = [
    "go",
    "run",
    "./scripts/atlas-schema",
  ]
}

variable "dev_url" {
  type    = string
  # 本地 Atlas dev database 默认跟随最新 pg_trgm 镜像；正式/受控环境建议固定具体版本或镜像 digest。
  default = "docker+postgres://_/aegiscore-atlas-postgres-pgtrgm:latest/dev?search_path=public"
}

env "local" {
  src     = data.external_schema.ent.url
  dev     = var.dev_url
  schemas = ["public"]

  migration {
    dir = "file://migrations"
  }
}

env "deploy" {
  url     = getenv("DATABASE_URL")
  schemas = ["public"]

  migration {
    dir = "file://migrations"
  }
}
