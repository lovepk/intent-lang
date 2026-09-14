// Build the npm distribution: cross-compile the Go `il` binary for each
// supported platform, drop it into a platform package, and sync versions in the
// main package. Run from anywhere:  node npm/scripts/build.mjs
//
// Env: VERSION (defaults to the main package's current version).

import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(here, "..", "..");
const mainPkgPath = join(repoRoot, "npm", "intent-lang", "package.json");
const platformsRoot = join(repoRoot, "npm", "platforms");

const mainPkg = JSON.parse(readFileSync(mainPkgPath, "utf8"));
const version = process.env.VERSION || mainPkg.version;

const targets = [
  { goos: "linux", goarch: "amd64", node: "linux-x64", os: "linux", cpu: "x64" },
  { goos: "linux", goarch: "arm64", node: "linux-arm64", os: "linux", cpu: "arm64" },
  { goos: "darwin", goarch: "amd64", node: "darwin-x64", os: "darwin", cpu: "x64" },
  { goos: "darwin", goarch: "arm64", node: "darwin-arm64", os: "darwin", cpu: "arm64" },
  { goos: "windows", goarch: "amd64", node: "win32-x64", os: "win32", cpu: "x64" },
  { goos: "windows", goarch: "arm64", node: "win32-arm64", os: "win32", cpu: "arm64" },
];

const optionalDependencies = {};

for (const t of targets) {
  const pkgName = `@lovepk/intent-lang-${t.node}`;
  optionalDependencies[pkgName] = version;

  const dir = join(platformsRoot, t.node);
  const binDir = join(dir, "bin");
  mkdirSync(binDir, { recursive: true });
  const exe = t.goos === "windows" ? "il.exe" : "il";
  const out = join(binDir, exe);

  console.log(`building ${t.node} (${t.goos}/${t.goarch}) -> ${out}`);
  execFileSync(
    "go",
    ["build", "-trimpath", "-ldflags", "-s -w", "-o", out, "./cmd/il"],
    {
      cwd: repoRoot,
      env: { ...process.env, GOOS: t.goos, GOARCH: t.goarch, CGO_ENABLED: "0" },
      stdio: "inherit",
    }
  );

  const pkg = {
    name: pkgName,
    version,
    description: `intent-lang CLI/MCP binary for ${t.node}`,
    os: [t.os],
    cpu: [t.cpu],
    files: ["bin"],
    license: "Apache-2.0",
    repository: {
      type: "git",
      url: "git+https://github.com/lovepk/intent-lang.git",
    },
  };
  writeFileSync(join(dir, "package.json"), JSON.stringify(pkg, null, 2) + "\n");
  writeFileSync(
    join(dir, "README.md"),
    `# ${pkgName}\n\nPlatform binary for [@lovepk/intent-lang](https://www.npmjs.com/package/@lovepk/intent-lang). Do not install directly.\n`
  );
}

// Keep the main package version + optional dependency versions in sync.
mainPkg.version = version;
mainPkg.optionalDependencies = optionalDependencies;
writeFileSync(mainPkgPath, JSON.stringify(mainPkg, null, 2) + "\n");

console.log(`\ndone: ${targets.length} platform packages @ ${version}`);
