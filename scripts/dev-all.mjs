import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import readline from "node:readline";
import net from "node:net";

const rootDir = process.cwd();
const backendDir = path.join(rootDir, "backend");

const loadEnv = (filePath) => {
  if (!fs.existsSync(filePath)) return;
  const contents = fs.readFileSync(filePath, "utf8");
  contents.split(/\r?\n/).forEach((line) => {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) return;
    const index = trimmed.indexOf("=");
    if (index === -1) return;
    const key = trimmed.slice(0, index).trim();
    let value = trimmed.slice(index + 1).trim();
    if (!key) return;
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    if (!process.env[key]) {
      process.env[key] = value;
    }
  });
};

const logPrefix = (label, line) => {
  const prefix = `[${label}]`;
  process.stdout.write(`${prefix} ${line}\n`);
};

const pipeOutput = (child, label) => {
  const handle = (stream) => {
    if (!stream) return;
    const rl = readline.createInterface({ input: stream });
    rl.on("line", (line) => logPrefix(label, line));
  };
  handle(child.stdout);
  handle(child.stderr);
};

const run = (label, command, args, options = {}) =>
  new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      shell: process.platform === "win32",
      ...options,
    });
    pipeOutput(child, label);
    child.on("close", (code) => {
      if (code === 0) resolve();
      else reject(new Error(`${label} exited with code ${code}`));
    });
  });

const runSilent = (command, args) =>
  new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      shell: process.platform === "win32",
      stdio: "ignore",
    });
    child.on("close", (code) => {
      if (code === 0) resolve();
      else reject(new Error(`${command} failed`));
    });
  });

const getGoPath = () =>
  new Promise((resolve) => {
    const child = spawn("go", ["env", "GOPATH"], {
      shell: process.platform === "win32",
      stdio: ["ignore", "pipe", "ignore"],
    });
    let output = "";
    if (child.stdout) {
      child.stdout.on("data", (chunk) => {
        output += chunk.toString();
      });
    }
    child.on("close", () => resolve(output.trim()));
  });

const ensureGoose = async () => {
  try {
    await runSilent("goose", ["-version"]);
    return;
  } catch {
    await run("migrate", "go", [
      "install",
      "github.com/pressly/goose/v3/cmd/goose@latest",
    ]);
    const goPath = await getGoPath();
    if (goPath) {
      process.env.PATH = `${path.join(goPath, "bin")}${path.delimiter}${process.env.PATH}`;
    }
    try {
      await runSilent("goose", ["-version"]);
    } catch {
      throw new Error(
        "goose is not available in PATH. Add GOPATH/bin to PATH."
      );
    }
  }
};

const checkDocker = async () => {
  try {
    await runSilent("docker", ["--version"]);
    await runSilent("docker", ["compose", "version"]);
  } catch {
    throw new Error(
      "Docker is not available. Install Docker Desktop and ensure 'docker' is in PATH."
    );
  }
};

const waitForDb = async (databaseUrl) => {
  if (!databaseUrl) throw new Error("DATABASE_URL is not set.");
  const url = new URL(databaseUrl);
  const host = url.hostname || "localhost";
  const port = Number(url.port || "5432");
  const maxAttempts = 30;

  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    const ready = await new Promise((resolve) => {
      const socket = net.createConnection({ host, port });
      socket.on("connect", () => {
        socket.end();
        resolve(true);
      });
      socket.on("error", () => {
        resolve(false);
      });
    });

    if (ready) return;
    logPrefix("db", `waiting for postgres... (${attempt}/${maxAttempts})`);
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }

  throw new Error("Postgres did not become ready in time.");
};

const startLongRunning = (label, command, args, options = {}) => {
  const child = spawn(command, args, {
    shell: process.platform === "win32",
    ...options,
  });
  pipeOutput(child, label);
  return child;
};

const main = async () => {
  loadEnv(path.join(rootDir, ".env"));
  const backendPort = process.env.PORT || "8080";
  const frontendPort = process.env.FRONTEND_PORT || "3000";

  await checkDocker();
  await ensureGoose();

  await run("db", "docker", ["compose", "up", "-d", "db"]);
  await waitForDb(process.env.DATABASE_URL);

  if (!fs.existsSync(path.join(rootDir, "node_modules"))) {
    await run("frontend", "npm", ["install"], { cwd: rootDir });
  }

  if (!fs.existsSync(path.join(backendDir, "go.sum"))) {
    await run("backend", "go", ["mod", "tidy"], { cwd: backendDir });
  }

  if (!process.env.DATABASE_URL) {
    throw new Error("DATABASE_URL is not set. Check your .env file.");
  }

  const migrateEnv = {
    ...process.env,
    GOOSE_DRIVER: "postgres",
    GOOSE_DBSTRING: process.env.DATABASE_URL,
  };

  await run("migrate", "goose", ["-dir", "backend/migrations", "up"], {
    cwd: rootDir,
    env: migrateEnv,
  });

  const backend = startLongRunning("backend", "go", ["run", "./cmd/server"], {
    cwd: backendDir,
    env: { ...process.env, PORT: backendPort },
  });
  const frontend = startLongRunning("frontend", "npm", ["run", "dev"], {
    cwd: rootDir,
    env: { ...process.env, PORT: frontendPort },
  });

  const shutdown = () => {
    backend.kill("SIGINT");
    frontend.kill("SIGINT");
  };

  process.on("SIGINT", () => {
    logPrefix("dev", "shutting down...");
    shutdown();
  });
  process.on("SIGTERM", shutdown);
};

main().catch((error) => {
  logPrefix("dev", error.message);
  process.exit(1);
});
