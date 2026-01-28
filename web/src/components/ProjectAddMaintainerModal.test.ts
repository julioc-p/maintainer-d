import assert from "node:assert/strict";
import { test } from "node:test";

import { inferNameFromRefLine } from "./ProjectAddMaintainerModal";

test("inferNameFromRefLine uses the table cell before the handle", () => {
  const line = "| Darin McAdams | d-mcadams | SPIRL |";
  const name = inferNameFromRefLine(line, "d-mcadams");
  assert.equal(name, "Darin McAdams");
});

test("inferNameFromRefLine falls back to words near the handle", () => {
  const line = "Darin McAdams @d-mcadams";
  const name = inferNameFromRefLine(line, "d-mcadams");
  assert.equal(name, "Darin McAdams");
});
