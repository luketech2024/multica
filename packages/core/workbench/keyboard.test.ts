// @vitest-environment node
import { expect, it } from "vitest";
import { workbenchReadingOrder } from "./keyboard";

it("reads board rows left to right, skipping empty and shorter columns", () => {
  expect(workbenchReadingOrder([["a", "b", "c"], [], ["d", "e"], ["f"]])).toEqual(["a", "d", "f", "b", "e", "c"]);
  expect(workbenchReadingOrder([])).toEqual([]);
  expect(workbenchReadingOrder([["a", "b"]])).toEqual(["a", "b"]);
});
