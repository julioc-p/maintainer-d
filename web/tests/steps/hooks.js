const {
  BeforeAll,
  AfterAll,
  Before,
  After,
  setDefaultTimeout,
} = require("@cucumber/cucumber");
const { chromium } = require("playwright");
const fs = require("node:fs");
const path = require("node:path");

let browser;

setDefaultTimeout(20 * 1000);

BeforeAll(async () => {
  browser = await chromium.launch({ headless: true });
});

AfterAll(async () => {
  if (browser) {
    await browser.close();
  }
});

Before(async function () {
  const artifactsDir =
    process.env.WEB_TEST_ARTIFACTS_DIR || path.join(process.cwd(), "testdata", "web-artifacts");
  fs.mkdirSync(artifactsDir, { recursive: true });
  this.context = await browser.newContext({
    recordVideo: { dir: artifactsDir },
  });
  this.page = await this.context.newPage();
  this.artifactsDir = artifactsDir;
});

After(async function (scenario) {
  if (scenario?.result?.status === "FAILED") {
    const safeName = scenario.pickle.name.replace(/[^a-z0-9-_]+/gi, "_").slice(0, 80);
    await this.page.screenshot({
      path: path.join(this.artifactsDir, `${safeName}.png`),
      fullPage: true,
    });
  }
  if (this.page) {
    await this.page.close();
  }
  if (this.context) {
    await this.context.close();
  }
});
