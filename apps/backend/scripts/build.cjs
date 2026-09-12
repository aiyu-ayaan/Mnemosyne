const { execFileSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const backendDir = path.resolve(__dirname, "..");
const binDir = path.resolve(__dirname, "../../../bin");
const exeName = process.platform === "win32" ? "mnemosyne.exe" : "mnemosyne";
const target = path.join(binDir, exeName);
const oldTarget = path.join(binDir, `${exeName}.old`);

// Ensure bin directory exists
if (!fs.existsSync(binDir)) {
  fs.mkdirSync(binDir, { recursive: true });
}

// Clean up previous .old if not locked
try {
  if (fs.existsSync(oldTarget)) {
    fs.unlinkSync(oldTarget);
  }
} catch {}

// On Windows, if target exists, rename it to .old so go build can write target even if in use
if (process.platform === "win32" && fs.existsSync(target)) {
  try {
    fs.renameSync(target, oldTarget);
  } catch {}
}

try {
  execFileSync("go", ["build", "-o", target, "./cmd/mnemosyne"], {
    stdio: "inherit",
    cwd: backendDir,
  });
} catch (err) {
  // If build failed and we had renamed target, restore it
  if (!fs.existsSync(target) && fs.existsSync(oldTarget)) {
    try {
      fs.renameSync(oldTarget, target);
    } catch {}
  }
  process.exit(1);
}

// Clean up .old if possible
try {
  if (fs.existsSync(oldTarget)) {
    fs.unlinkSync(oldTarget);
  }
} catch {}
