const { setWorldConstructor } = require("@cucumber/cucumber");

class WebWorld {
  constructor() {
    this.browser = null;
    this.page = null;
    this.baseUrl = process.env.WEB_BASE_URL || "http://localhost:3000";
    this.bffBaseUrl = process.env.BFF_BASE_URL || "http://localhost:8000";
  }
}

setWorldConstructor(WebWorld);
