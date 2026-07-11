import { copyFileSync, mkdirSync, readdirSync, rmSync } from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..")
const templatesDir = path.join(root, "templates")
const assetsDir = path.join(root, "../internal/oauthpages/assets")

mkdirSync(assetsDir, { recursive: true })

for (const name of readdirSync(templatesDir)) {
  if (!name.endsWith(".html")) {
    continue
  }
  copyFileSync(path.join(templatesDir, name), path.join(assetsDir, name))
}

rmSync(path.join(assetsDir, "_"), { recursive: true, force: true })

for (const name of readdirSync(assetsDir)) {
  if (name.endsWith(".woff2")) {
    rmSync(path.join(assetsDir, name))
  }
}
