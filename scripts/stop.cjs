// Stops a running daemon so a rebuild can replace the binary it is holding.
//
// With --dev it stops only the development daemon, which is the one `pnpm dev`
// owns; without it, both, so `pnpm stop` means what it says.
const { execFileSync, execSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const devOnly = process.argv.includes("--dev");
const binPath = path.resolve(
  __dirname,
  "../bin",
  process.platform === "win32" ? "mnemosyne.exe" : "mnemosyne"
);

/** Asks the binary to shut a daemon down. MNEMOSYNE_DEV picks which one. */
function stop(dev) {
  if (!fs.existsSync(binPath)) return;
  const env = { ...process.env };
  if (dev) env.MNEMOSYNE_DEV = "1";
  else delete env.MNEMOSYNE_DEV;
  try {
    execFileSync(binPath, ["stop"], { stdio: "inherit", timeout: 5000, env });
  } catch {}
}

stop(true);
if (!devOnly) stop(false);

// A daemon that was killed rather than asked leaves the binary locked, which is
// the failure this script exists to prevent. In dev only the copy in bin/ is
// ours to kill: the installed one belongs to the user's real setup.
if (process.platform === "win32") {
  const clause = devOnly
    ? "$_.CommandLine -like '*daemon*' -and $_.Path -eq '" + binPath.replace(/'/g, "''") + "'"
    : "$_.CommandLine -like '*daemon*'";
  const script =
    "Get-CimInstance Win32_Process -Filter \"Name like 'mnemosyne%'\" | " +
    "Where-Object { " + clause + " } | " +
    "ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }";
  try {
    execSync("powershell -NoProfile -Command " + JSON.stringify(script), {
      stdio: "ignore",
      timeout: 10000,
    });
  } catch {}
}
