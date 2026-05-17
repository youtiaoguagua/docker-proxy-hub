import { access, cp, mkdir, rm } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const rootDir = path.dirname(fileURLToPath(import.meta.url));
const sourceDir = path.join(rootDir, "web", "dist");
const targetDir = path.join(rootDir, "internal", "frontend", "dist");

await access(sourceDir);
await rm(targetDir, { force: true, recursive: true });
await mkdir(targetDir, { recursive: true });
await cp(sourceDir, targetDir, { recursive: true });

process.stdout.write(`synced ${sourceDir} -> ${targetDir}\n`);
