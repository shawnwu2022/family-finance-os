#!/usr/bin/env node
import fs from "node:fs";

function fail(message) {
  throw new Error(message);
}

const [path, serverName, toolName, marker, maxCallsRaw] = process.argv.slice(2);
if (!path || !serverName || !toolName || !marker || !maxCallsRaw) {
  fail("usage: openclaw-agent-result-validator.mjs <result.json> <server> <tool> <marker> <max-calls>");
}

const maxCalls = Number(maxCallsRaw);
if (!Number.isInteger(maxCalls) || maxCalls < 1) {
  fail("max-calls must be a positive integer");
}

const payload = JSON.parse(fs.readFileSync(path, "utf8"));
const meta = payload && typeof payload.meta === "object" && payload.meta !== null ? payload.meta : {};
const expectedToolName = `${serverName}__${toolName}`;

if (meta.aborted === true) {
  fail("agent result is aborted");
}
if (meta.error) {
  fail("agent result contains an error");
}

const systemPromptReport =
  meta && typeof meta.systemPromptReport === "object" && meta.systemPromptReport !== null
    ? meta.systemPromptReport
    : {};
const reportTools =
  systemPromptReport && typeof systemPromptReport.tools === "object" && systemPromptReport.tools !== null
    ? systemPromptReport.tools
    : {};
const runtimeToolNames = Array.isArray(reportTools.entries)
  ? reportTools.entries
      .map((entry) => (entry && typeof entry.name === "string" ? entry.name : ""))
      .filter(Boolean)
  : [];
if (runtimeToolNames.length !== 1 || runtimeToolNames[0] !== expectedToolName) {
  fail(`model-facing tool surface is not exactly ${expectedToolName}`);
}

const finalAssistantVisibleText =
  typeof meta.finalAssistantVisibleText === "string" ? meta.finalAssistantVisibleText.trim() : "";
if (finalAssistantVisibleText !== marker) {
  fail("stable meta.finalAssistantVisibleText does not match the acceptance marker");
}

const toolSummary =
  meta && typeof meta.toolSummary === "object" && meta.toolSummary !== null ? meta.toolSummary : null;
if (!toolSummary) {
  fail("stable meta.toolSummary is missing");
}

const calls = toolSummary.calls;
if (!Number.isInteger(calls) || calls < 1 || calls > maxCalls) {
  fail(`toolSummary.calls ${calls} is outside accepted range 1..${maxCalls}`);
}
if (
  !Array.isArray(toolSummary.tools) ||
  toolSummary.tools.length !== 1 ||
  toolSummary.tools[0] !== expectedToolName
) {
  fail(`toolSummary.tools does not contain only ${expectedToolName}`);
}
if (Number(toolSummary.failures ?? 0) !== 0) {
  fail("toolSummary.failures is non-zero");
}

console.log(`OpenClaw agent result valid: tool=${expectedToolName} calls=${calls}`);
