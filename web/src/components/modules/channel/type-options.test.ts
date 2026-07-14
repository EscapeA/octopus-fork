import assert from "node:assert/strict";
import test from "node:test";

import { CHANNEL_TYPE_OPTIONS, DEFAULT_CHANNEL_TYPE } from "./type-options.ts";

const OpenAIChat: number = 0;
const OpenAIResponse: number = 1;

test("channel type options merge OpenAI chat and response into one OpenAI option", () => {
  const openaiOptions = CHANNEL_TYPE_OPTIONS.filter(
    (option) => option.value === OpenAIChat || option.value === OpenAIResponse,
  );

  assert.deepEqual(openaiOptions, [
    { value: OpenAIResponse, labelKey: "typeOpenAI" },
  ]);
});

test("DEFAULT_CHANNEL_TYPE matches the first option in CHANNEL_TYPE_OPTIONS (issue #143)", () => {
  assert.equal(DEFAULT_CHANNEL_TYPE, CHANNEL_TYPE_OPTIONS[0].value);
});
