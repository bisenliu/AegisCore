data "external_schema" "ent" {
  program = [
    "go",
    "run",
    "./scripts/atlas-schema",
  ]
}

variable "dev_url" {
  type    = string
  default = "docker+postgres://_/aegiscore-atlas-postgres-pgtrgm:15/dev?search_path=public"
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
