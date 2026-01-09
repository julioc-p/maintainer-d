module.exports = {
  default: [
    "../features/web/**/*.feature",
    "--require",
    "tests/steps/**/*.js",
    "--tags",
    "not @wip",
    "--format",
    "junit:../testdata/web-bdd-results.xml",
    "--format",
    "json:../testdata/web-bdd-report.json",
    "--format",
    "@cucumber/pretty-formatter",
  ],
};
