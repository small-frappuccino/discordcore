import { mkdir, copyFile } from "node:fs/promises";
import { join } from "node:path";

const root = new URL("..", import.meta.url).pathname;
const clientDir = join(root, "client");
const distDir = join(clientDir, "dist");
const staticDir = join(distDir, "static");

await mkdir(staticDir, { recursive: true });

const result = await Bun.build({
  entrypoints: [join(clientDir, "src", "main.tsx")],
  outdir: staticDir,
  minify: true,
  sourcemap: "external",
  target: "browser",
});

if (!result.success) {
  console.error(result.logs);
  process.exit(1);
}

await copyFile(join(clientDir, "index.html"), join(distDir, "index.html"));
await copyFile(join(clientDir, "src", "styles.css"), join(staticDir, "styles.css"));
