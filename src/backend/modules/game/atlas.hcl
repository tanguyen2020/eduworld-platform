# Atlas project of module game (ADR-0039). Migrations are versioned and reviewed; never auto-applied.
# URLs come from the environment so no credential lives in the repository.

variable "url" {
  type    = string
  default = getenv("EW_DB_GAME_MIGRATOR_DSN")
}

variable "dev_url" {
  type    = string
  default = getenv("EW_ATLAS_DEV_URL")
}

env "local" {
  url = var.url
  dev = var.dev_url
  migration {
    dir = "file://modules/game/migrations"
  }
}
