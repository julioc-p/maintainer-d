const { When, Then } = require("@cucumber/cucumber");
const { expect } = require("@playwright/test");

When("I search for {string}", async function (query) {
  const searchInput = this.page.getByPlaceholder("Search projects");
  await searchInput.fill(query);
  await searchInput.press("Enter");
});

When("I click on maintainer {string}", async function (name) {
  await Promise.all([
    this.page.waitForURL(/\/maintainers\/\d+/, { timeout: 10000 }),
    this.page.getByRole("link", { name }).first().click(),
  ]);
});

Then("I see the maintainer card for {string}", async function (name) {
  await Promise.race([
    this.page.getByRole("heading", { name }).waitFor({ timeout: 15000 }),
    this.page.getByText("Unable to load maintainer").waitFor({ timeout: 15000 }),
  ]);
  await expect(this.page.getByRole("heading", { name })).toBeVisible({
    timeout: 15000,
  });
});
