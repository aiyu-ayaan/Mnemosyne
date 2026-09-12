const { execFileSync, execSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const binPath = path.resolve(
  __dirname,
  "../bin",
  process.platform === "win32" ? "mnemosyne.exe" : "mnemosyne"
);

// 1. If the binary exists, run mnemosyne stop (which calls /v1/shutdown and stops service)
if (fs.existsSync(binPath)) {
  try {
    execFileSync(binPath, ["stop"], { stdio: "inherit", timeout: 5000 });
  } catch {}
}

// 2. On Windows, ensure any lingering detached daemon processes are cleaned up (leave serve processes intact)
if (process.platform === "win32") {
  try {
    execSync(
      'powershell -NoProfile -Command "Get-CimInstance Win32_Process -Filter \\"Name like \'mnemosyne%\'\\" | Where-Object { $_.CommandLine -like \'*daemon*\' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }"',
      { stdio: "ignore", timeout: 5000 }
    );
  } catch {}
}
