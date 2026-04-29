import { execFile } from "child_process";
import { promisify } from "util";
import * as os from "os";
import * as path from "path";

const execFileAsync = promisify(execFile);

export interface Entry {
  path: string;
  name: string;
  type: "file" | "directory" | "symlink" | "other";
  logical_size: number;
  physical_size: number;
  cluster_size: number;
  is_symlink: boolean;
  is_cloud: boolean;
  cloud_provider?: string;
  cloud_status?: string;
  mod_time: string;
  permissions: number;
  children?: Entry[];
}

export interface ScanConfig {
  rootPath: string;
  showHidden: boolean;
  excludePatterns: string[];
  followSymlinks: boolean;
  maxDepth: number;
}

async function fileExists(executablePath: string): Promise<boolean> {
  try {
    await execFileAsync("/bin/test", ["-x", executablePath]);
    return true;
  } catch {
    return false;
  }
}

export async function findBinary(): Promise<string> {
  const candidates = [
    path.join(__dirname, "..", "..", "..", "bin", "rdm-cli"),
    "/usr/local/bin/rdm-cli",
    "/opt/homebrew/bin/rdm-cli",
    path.join(os.homedir(), ".local", "bin", "rdm-cli"),
    path.join(os.homedir(), "bin", "rdm-cli"),
  ];

  for (const candidate of candidates) {
    if (await fileExists(candidate)) {
      return candidate;
    }
  }

  try {
    const { stdout } = await execFileAsync("which", ["rdm-cli"]);
    const found = stdout.trim();
    if (found) {
      return found;
    }
  } catch {
    // Ignore PATH lookup failures.
  }

  throw new Error("rdm-cli binary not found. Build/install it first (make build && make install).");
}

export function expandHome(inputPath: string): string {
  if (!inputPath || inputPath === "~") {
    return os.homedir();
  }
  if (inputPath.startsWith("~/")) {
    return path.join(os.homedir(), inputPath.slice(2));
  }
  return inputPath;
}

export async function scanDirectory(config: ScanConfig): Promise<Entry> {
  const binary = await findBinary();
  const args: string[] = ["-json", "-root", expandHome(config.rootPath)];

  if (config.showHidden) {
    args.push("-hidden");
  }
  if (config.followSymlinks) {
    args.push("-follow-symlinks");
  }
  if (config.maxDepth > 0) {
    args.push("-max-depth", String(config.maxDepth));
  }
  if (config.excludePatterns.length > 0) {
    args.push("-exclude", config.excludePatterns.join(","));
  }

  const { stdout, stderr } = await execFileAsync(binary, args, {
    timeout: 10 * 60 * 1000,
    maxBuffer: 1024 * 1024 * 64,
  });

  if (stderr?.trim()) {
    console.error(stderr);
  }

  return JSON.parse(stdout) as Entry;
}

export async function revealInFinder(targetPath: string): Promise<void> {
  await execFileAsync("open", ["-R", targetPath]);
}

export async function openInFinder(targetPath: string): Promise<void> {
  await execFileAsync("open", [targetPath]);
}

export async function openInTerminal(targetPath: string): Promise<void> {
  const script = `tell application \"Terminal\" to do script \"cd '${targetPath.replace(/'/g, `'\\''`)}'\"`;
  await execFileAsync("osascript", ["-e", script]);
}
