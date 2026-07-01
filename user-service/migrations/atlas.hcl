variable "dev_url" {
  type    = string
  default = "docker+postgres://_/aegiscore-atlas-postgres-pgtrgm:15/dev?search_path=public"
}

env "local" {
  src     = "ent://ent/schema"
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
