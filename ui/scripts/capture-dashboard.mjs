import { mkdir, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import process from "node:process";
import readline from "node:readline/promises";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright-core";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const uiDir = resolve(scriptDir, "..");
const defaultOutputRoot = resolve(uiDir, "debug-screenshots");
const defaultProfileDir = resolve(defaultOutputRoot, "profile");
const defaultBaseUrl =
  process.env.DASHBOARD_CAPTURE_BASE_URL ?? "https://alice.localhost:8443";

const captureTargets = [
  {
    id: "home",
    label: "Home",
    path: "/dashboard/home",
    expectations: {
      headings: [
        { name: "Home", level: 1 },
        { name: "Main modules", level: 2 },
        { name: "Current blockers", level: 2 },
        { name: "Quick shortcuts", level: 2 },
        { name: "Advanced stays in Settings", level: 2 },
      ],
      absentHeadings: [{ name: "Maintenance", level: 2 }],
      absentTexts: ["Advanced controls > Maintenance"],
    },
  },
  {
    id: "partner-board-entries",
    label: "Partner Board Entries",
    path: "/dashboard/partner-board/entries",
  },
  {
    id: "partner-board-layout",
    label: "Partner Board Layout",
    path: "/dashboard/partner-board/layout",
  },
  {
    id: "partner-board-delivery",
    label: "Partner Board Delivery",
    path: "/dashboard/partner-board/delivery",
  },
  { id: "commands", label: "Commands", path: "/dashboard/commands" },
  { id: "moderation", label: "Moderation", path: "/dashboard/moderation" },
  { id: "logging", label: "Logging", path: "/dashboard/logging" },
  { id: "roles", label: "Roles", path: "/dashboard/roles" },
  { id: "stats", label: "Stats", path: "/dashboard/stats" },
  {
    id: "settings-server",
    label: "Settings Server",
    path: "/dashboard/settings",
  },
  {
    id: "settings-connection",
    label: "Settings Connection",
    path: "/dashboard/settings#connection",
  },
  {
    id: "settings-permissions",
    label: "Settings Permissions",
    path: "/dashboard/settings#permissions",
  },
  {
    id: "settings-advanced",
    label: "Settings Advanced",
    path: "/dashboard/settings#advanced",
  },
  {
    id: "settings-diagnostics",
    label: "Settings Diagnostics",
    path: "/dashboard/settings#diagnostics",
  },
];

async function main() {
  const options = parseArgs(process.argv.slice(2));
  if (options.help) {
    printHelp();
    return;
  }

  const selectedTargets = resolveTargets(options.only);
  const executablePath = resolveBrowserExecutable(
    options.browserPath,
    options.browser,
  );

  if (executablePath === null) {
    throw new Error(
      `Could not find a ${options.browser} executable. Pass --browser-path="C:\\Path\\To\\chrome.exe" to continue.`,
    );
  }

  const runDir =
    options.outputDir === null
      ? resolve(defaultOutputRoot, timestampDirectoryName())
      : resolve(uiDir, options.outputDir);

  await mkdir(runDir, { recursive: true });
  await mkdir(options.profileDir, { recursive: true });

  console.log(`Using browser: ${executablePath}`);
  console.log(`Using profile: ${options.profileDir}`);
  console.log(`Saving screenshots to: ${runDir}`);
  console.log(`Capturing ${selectedTargets.length} route(s) from ${options.baseUrl}`);

  const context = await chromium.launchPersistentContext(options.profileDir, {
    executablePath,
    headless: options.headless,
    viewport: {
      width: options.width,
      height: options.height,
    },
    deviceScaleFactor: options.deviceScaleFactor,
    ignoreHTTPSErrors: true,
    args: [
      `--window-size=${options.width},${options.height}`,
      "--hide-crash-restore-bubble",
      "--disable-session-crashed-bubble",
    ],
  });

  const page = context.pages()[0] ?? (await context.newPage());
  const manifest = {
    created_at: new Date().toISOString(),
    base_url: options.baseUrl,
    viewport: {
      width: options.width,
      height: options.height,
      device_scale_factor: options.deviceScaleFactor,
    },
    browser: {
      name: options.browser,
      executable_path: executablePath,
      headless: options.headless,
      profile_dir: options.profileDir,
    },
    output_dir: runDir,
    routes: [],
  };

  try {
    if (options.interactive) {
      console.log("");
      console.log("Interactive mode enabled.");
      console.log(
        "Sign in, choose the target server, and leave the dashboard on a ready state before continuing.",
      );
      await navigate(page, buildTargetUrl(options.baseUrl, selectedTargets[0].path), options);
      await waitForEnter("Press Enter to start the capture run...");
    }

    for (const target of selectedTargets) {
      const url = buildTargetUrl(options.baseUrl, target.path);
      const screenshotPath = resolve(runDir, `${target.id}.png`);

      console.log(`Capturing ${target.label} -> ${screenshotPath}`);

      const routeEntry = {
        id: target.id,
        label: target.label,
        path: target.path,
        url,
        file: screenshotPath,
        status: "ok",
      };

      try {
        await navigate(page, url, options);
        await verifyCaptureTarget(page, target);
        await page.screenshot({
          path: screenshotPath,
          fullPage: true,
          animations: "disabled",
        });
      } catch (error) {
        routeEntry.status = "error";
        routeEntry.error = formatError(error);
        console.error(`Failed to capture ${target.label}: ${routeEntry.error}`);
      }

      manifest.routes.push(routeEntry);
    }
  } finally {
    await context.close();
  }

  const manifestPath = resolve(runDir, "manifest.json");
  await writeFile(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`, "utf8");
  console.log(`Saved manifest: ${manifestPath}`);
}

async function navigate(page, url, options) {
  await page.goto(url, {
    waitUntil: "domcontentloaded",
    timeout: options.timeoutMs,
  });

  try {
    await page.waitForLoadState("networkidle", {
      timeout: Math.min(options.timeoutMs, 15_000),
    });
  } catch {
    // Some dashboard routes keep background requests active; continue after the grace period.
  }

  await page.evaluate(() => {
    return new Promise((resolve) => {
      requestAnimationFrame(() => {
        requestAnimationFrame(resolve);
      });
    });
  });

  if (options.waitMs > 0) {
    await page.waitForTimeout(options.waitMs);
  }
}

async function verifyCaptureTarget(page, target) {
  if (!target.expectations) {
    return;
  }

  await assertCurrentPath(page, target.path);

  for (const heading of target.expectations.headings ?? []) {
    await page
      .getByRole("heading", {
        name: heading.name,
        level: heading.level,
      })
      .waitFor({
        state: "visible",
        timeout: 5_000,
      });
  }

  for (const heading of target.expectations.absentHeadings ?? []) {
    const count = await page
      .getByRole("heading", {
        name: heading.name,
        level: heading.level,
      })
      .count();
    if (count > 0) {
      throw new Error(
        `Capture verification failed for ${target.id}: unexpected heading "${heading.name}" is still visible.`,
      );
    }
  }

  for (const text of target.expectations.absentTexts ?? []) {
    const count = await page.getByText(text, { exact: true }).count();
    if (count > 0) {
      throw new Error(
        `Capture verification failed for ${target.id}: unexpected legacy text "${text}" is still visible.`,
      );
    }
  }
}

async function assertCurrentPath(page, expectedPath) {
  const currentUrl = new URL(page.url());
  const currentPath = `${currentUrl.pathname}${currentUrl.hash}`;

  if (currentPath !== expectedPath) {
    throw new Error(
      `Capture verification failed: expected route ${expectedPath}, but the browser is on ${currentPath}. Rebuild and restart the host if it is serving stale embedded assets.`,
    );
  }
}

function parseArgs(args) {
  const options = {
    baseUrl: defaultBaseUrl,
    outputDir: null,
    profileDir: defaultProfileDir,
    width: 1920,
    height: 1080,
    deviceScaleFactor: 1,
    waitMs: 1200,
    timeoutMs: 45_000,
    browser: "chrome",
    browserPath: "",
    headless: true,
    interactive: false,
    only: "",
    help: false,
  };

  for (const rawArg of args) {
    if (rawArg === "--help" || rawArg === "-h") {
      options.help = true;
      continue;
    }
    if (rawArg === "--headed") {
      options.headless = false;
      continue;
    }
    if (rawArg === "--interactive") {
      options.interactive = true;
      options.headless = false;
      continue;
    }
    if (!rawArg.startsWith("--")) {
      throw new Error(`Unexpected argument: ${rawArg}`);
    }

    const [key, ...valueParts] = rawArg.slice(2).split("=");
    const value = valueParts.join("=");

    switch (key) {
      case "base-url":
        options.baseUrl = value;
        break;
      case "output-dir":
        options.outputDir = value;
        break;
      case "profile-dir":
        options.profileDir = resolve(uiDir, value);
        break;
      case "width":
        options.width = parsePositiveInteger(key, value);
        break;
      case "height":
        options.height = parsePositiveInteger(key, value);
        break;
      case "device-scale-factor":
        options.deviceScaleFactor = parsePositiveNumber(key, value);
        break;
      case "wait-ms":
        options.waitMs = parseNonNegativeInteger(key, value);
        break;
      case "timeout-ms":
        options.timeoutMs = parsePositiveInteger(key, value);
        break;
      case "browser":
        options.browser = value || options.browser;
        break;
      case "browser-path":
        options.browserPath = value;
        break;
      case "only":
        options.only = value;
        break;
      default:
        throw new Error(`Unknown argument: --${key}`);
    }
  }

  return options;
}

function resolveTargets(onlyValue) {
  if (onlyValue.trim() === "") {
    return captureTargets;
  }

  const requestedIds = onlyValue
    .split(",")
    .map((value) => value.trim())
    .filter(Boolean);
  const requestedIdSet = new Set(requestedIds);
  const selectedTargets = captureTargets.filter((target) =>
    requestedIdSet.has(target.id),
  );

  if (selectedTargets.length !== requestedIds.length) {
    const knownIds = new Set(captureTargets.map((target) => target.id));
    const unknownIds = requestedIds.filter((id) => !knownIds.has(id));
    throw new Error(
      `Unknown route id(s): ${unknownIds.join(", ")}. Use --help to list the available targets.`,
    );
  }

  return selectedTargets;
}

function buildTargetUrl(baseUrl, path) {
  const normalizedBaseUrl = baseUrl.endsWith("/") ? baseUrl : `${baseUrl}/`;
  return new URL(path.startsWith("/") ? path.slice(1) : path, normalizedBaseUrl).toString();
}

function resolveBrowserExecutable(explicitPath, browser) {
  if (explicitPath.trim() !== "") {
    return resolve(explicitPath);
  }

  const candidates = getBrowserCandidates(browser);
  return candidates.find((candidate) => existsSync(candidate)) ?? null;
}

function getBrowserCandidates(browser) {
  const localAppData = process.env.LOCALAPPDATA ?? "";
  const programFiles = process.env.ProgramFiles ?? "";
  const programFilesX86 = process.env["ProgramFiles(x86)"] ?? "";
  const home = process.env.HOME ?? process.env.USERPROFILE ?? "";

  if (process.platform === "win32") {
    switch (browser) {
      case "msedge":
        return [
          resolve(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
          resolve(
            programFilesX86,
            "Microsoft",
            "Edge",
            "Application",
            "msedge.exe",
          ),
          resolve(localAppData, "Microsoft", "Edge", "Application", "msedge.exe"),
        ];
      case "chrome":
      default:
        return [
          resolve(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
          resolve(
            programFilesX86,
            "Google",
            "Chrome",
            "Application",
            "chrome.exe",
          ),
          resolve(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
        ];
    }
  }

  if (process.platform === "darwin") {
    if (browser === "msedge") {
      return [
        "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
        resolve(
          home,
          "Applications",
          "Microsoft Edge.app",
          "Contents",
          "MacOS",
          "Microsoft Edge",
        ),
      ];
    }

    return [
      "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
      resolve(
        home,
        "Applications",
        "Google Chrome.app",
        "Contents",
        "MacOS",
        "Google Chrome",
      ),
    ];
  }

  if (browser === "msedge") {
    return ["/usr/bin/microsoft-edge", "/usr/bin/microsoft-edge-stable"];
  }

  return [
    "/usr/bin/google-chrome",
    "/usr/bin/google-chrome-stable",
    "/snap/bin/chromium",
  ];
}

function timestampDirectoryName() {
  return new Date().toISOString().replaceAll(":", "-");
}

function parsePositiveInteger(key, value) {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`--${key} must be a positive integer. Received: ${value}`);
  }
  return parsed;
}

function parseNonNegativeInteger(key, value) {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isInteger(parsed) || parsed < 0) {
    throw new Error(`--${key} must be a non-negative integer. Received: ${value}`);
  }
  return parsed;
}

function parsePositiveNumber(key, value) {
  const parsed = Number.parseFloat(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    throw new Error(`--${key} must be a positive number. Received: ${value}`);
  }
  return parsed;
}

function formatError(error) {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

async function waitForEnter(prompt) {
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
  });

  try {
    await rl.question(`${prompt}\n`);
  } finally {
    rl.close();
  }
}

function printHelp() {
  console.log(`Capture full-page dashboard screenshots into ui/debug-screenshots.

Usage:
  bun run capture:dashboard
  bun run capture:dashboard -- --headed
  bun run capture:dashboard -- --interactive
  bun run capture:dashboard -- --base-url=https://alice.localhost:8443 --only=home,commands

Options:
  --base-url=URL               Dashboard origin. Default: ${defaultBaseUrl}
  --output-dir=PATH            Output directory relative to ui/. Default: debug-screenshots/<timestamp>
  --profile-dir=PATH           Persistent browser profile relative to ui/. Default: debug-screenshots/profile
  --width=NUMBER               Viewport width. Default: 1920
  --height=NUMBER              Viewport height. Default: 1080
  --device-scale-factor=NUM    Device scale factor. Default: 1
  --wait-ms=NUMBER             Extra wait after each navigation. Default: 1200
  --timeout-ms=NUMBER          Navigation timeout. Default: 45000
  --browser=chrome|msedge      Installed browser to drive. Default: chrome
  --browser-path=PATH          Explicit browser executable path
  --headed                     Show the browser window instead of running headless
  --interactive                Show the browser and wait for Enter before capture
  --only=id1,id2               Capture a subset of routes
  --help                       Show this help

Available route ids:
  ${captureTargets.map((target) => `${target.id} (${target.path})`).join("\n  ")}

Recommended first run:
  bun run capture:dashboard -- --interactive

That stores the authenticated session in the persistent profile so later runs can stay fully automatic.

If the capture host is Alicebot on https://alice.localhost:8443, rebuild ui/dist
and restart Alicebot before capturing. The script verifies the expected Home
surface and now fails fast when the host still serves stale embedded assets.
`);
}

main().catch((error) => {
  console.error(formatError(error));
  process.exitCode = 1;
});
