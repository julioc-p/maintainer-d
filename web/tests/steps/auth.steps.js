const { Given, When } = require("@cucumber/cucumber");

Given("I am signed in as staff", async function () {
  const login = process.env.TEST_STAFF_LOGIN || "staff-tester";
  await this.page.goto(
    `${this.bffBaseUrl}/auth/test-login?login=${encodeURIComponent(login)}`,
    { waitUntil: "domcontentloaded" }
  );
  await this.page.goto(this.baseUrl);
  this.currentRole = "staff";
});

When("the user signs in with GitHub OIDC using that account", async function () {
  const login = this.currentLogin || process.env.TEST_STAFF_LOGIN || "staff-tester";
  await this.page.goto(
    `${this.bffBaseUrl}/auth/test-login?login=${encodeURIComponent(login)}`,
    { waitUntil: "domcontentloaded" }
  );
  await this.page.goto(this.baseUrl);
});
