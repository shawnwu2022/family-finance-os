#!/usr/bin/env node
import { createHash } from "node:crypto";
import { appendFileSync, openSync, closeSync } from "node:fs";
import http from "node:http";

const listenAddress = process.env.OLLAMA_PROXY_LISTEN ?? "127.0.0.1:11435";
const upstreamBase = new URL(process.env.OLLAMA_PROXY_UPSTREAM ?? "http://127.0.0.1:11434");
const diagFile = process.env.OLLAMA_PROXY_DIAG_FILE;
const workflowDiagFile = process.env.GITHUB_ACTIONS === "true"
  ? "/tmp/family-finance-ollama-boundary-safe.jsonl"
  : undefined;
const maxRequestBytes = 8 * 1024 * 1024;

if (!diagFile) {
  throw new Error("OLLAMA_PROXY_DIAG_FILE is required");
}

const separator = listenAddress.lastIndexOf(":");
if (separator <= 0) {
  throw new Error("OLLAMA_PROXY_LISTEN must be host:port");
}
const listenHost = listenAddress.slice(0, separator);
const listenPort = Number.parseInt(listenAddress.slice(separator + 1), 10);
if (!Number.isInteger(listenPort) || listenPort <= 0 || listenPort > 65535) {
  throw new Error("OLLAMA_PROXY_LISTEN port is invalid");
}

const diagFd = openSync(diagFile, "a", 0o600);
const workflowDiagFd = workflowDiagFile ? openSync(workflowDiagFile, "a", 0o600) : undefined;
let sequence = 0;

function sha256Json(value) {
  return createHash("sha256").update(JSON.stringify(value ?? null), "utf8").digest("hex");
}

function contentChars(value) {
  if (typeof value === "string") return value.length;
  if (value == null) return 0;
  try {
    return JSON.stringify(value).length;
  } catch {
    return -1;
  }
}

function safeOptionSummary(value) {
  if (!value || typeof value !== "object" || Array.isArray(value)) return {};
  const allowed = new Set([
    "temperature",
    "num_predict",
    "num_ctx",
    "top_p",
    "top_k",
    "min_p",
    "seed",
    "repeat_penalty",
    "presence_penalty",
    "frequency_penalty",
  ]);
  const summary = {};
  for (const [key, nested] of Object.entries(value)) {
    if (!allowed.has(key)) continue;
    if (typeof nested === "number" || typeof nested === "boolean") {
      summary[key] = nested;
    }
  }
  return summary;
}

function requestSummary(requestId, path, body) {
  if (path !== "/api/chat" || !body || typeof body !== "object" || Array.isArray(body)) {
    return undefined;
  }
  const messages = Array.isArray(body.messages) ? body.messages : [];
  const tools = Array.isArray(body.tools) ? body.tools : [];
  const toolNames = tools.map((entry) => entry?.function?.name).filter((name) => typeof name === "string");
  const toolSchemaSha256 = tools.map((entry) => sha256Json(entry?.function?.parameters));
  const systemMessages = messages.filter((entry) => entry?.role === "system");
  const options = safeOptionSummary(body.options);

  return {
    requestId,
    direction: "request",
    path,
    model: typeof body.model === "string" ? body.model : null,
    stream: body.stream === true,
    think: typeof body.think === "boolean" || typeof body.think === "string" ? body.think : null,
    topLevelKeys: Object.keys(body).sort(),
    messageCount: messages.length,
    messageRoles: messages.map((entry) => (typeof entry?.role === "string" ? entry.role : null)),
    messageContentChars: messages.map((entry) => contentChars(entry?.content)),
    systemMessageCount: systemMessages.length,
    systemMessageChars: systemMessages.map((entry) => contentChars(entry?.content)),
    toolCount: tools.length,
    toolNames,
    toolSchemaSha256,
    optionKeys: Object.keys(body.options ?? {}).sort(),
    options,
  };
}

function createResponseObserver(requestId, path, status) {
  let carry = "";
  let chunkCount = 0;
  let responseToolCallCount = 0;
  const responseToolNames = [];
  const doneReasons = [];

  function observeObject(value) {
    if (!value || typeof value !== "object" || Array.isArray(value)) return;
    const calls = Array.isArray(value?.message?.tool_calls) ? value.message.tool_calls : [];
    for (const call of calls) {
      responseToolCallCount += 1;
      const name = call?.function?.name;
      if (typeof name === "string") responseToolNames.push(name);
    }
    const reason = value.done_reason;
    if (typeof reason === "string" && reason) doneReasons.push(reason);
  }

  return {
    observe(chunk) {
      chunkCount += 1;
      carry += chunk.toString("utf8");
      for (;;) {
        const newline = carry.indexOf("\n");
        if (newline < 0) break;
        const line = carry.slice(0, newline).trim();
        carry = carry.slice(newline + 1);
        if (!line) continue;
        try {
          observeObject(JSON.parse(line));
        } catch {
          // A partial/non-NDJSON body is handled at end when possible.
        }
      }
    },
    finish() {
      const tail = carry.trim();
      if (tail) {
        try {
          observeObject(JSON.parse(tail));
        } catch {
          // The proxy reports only parseable structured Ollama metadata.
        }
      }
      return {
        requestId,
        direction: "response",
        path,
        status,
        chunkCount,
        responseToolCallCount,
        responseToolNames,
        doneReasons,
      };
    },
  };
}

function emit(summary) {
  const line = JSON.stringify(summary);
  appendFileSync(diagFd, `${line}\n`, "utf8");
  if (workflowDiagFd !== undefined) {
    appendFileSync(workflowDiagFd, `${line}\n`, "utf8");
  }
  console.log(`ollama_boundary_diag ${line}`);
}

const server = http.createServer((clientReq, clientRes) => {
  const requestId = ++sequence;
  const path = clientReq.url ?? "/";
  const chunks = [];
  let size = 0;
  let rejected = false;

  clientReq.on("data", (chunk) => {
    if (rejected) return;
    size += chunk.length;
    if (size > maxRequestBytes) {
      rejected = true;
      clientRes.writeHead(413, { "content-type": "text/plain" });
      clientRes.end("request too large");
      clientReq.destroy();
      return;
    }
    chunks.push(chunk);
  });

  clientReq.on("end", () => {
    if (rejected) return;
    const bodyBuffer = Buffer.concat(chunks);
    let parsedBody;
    if (bodyBuffer.length > 0) {
      try {
        parsedBody = JSON.parse(bodyBuffer.toString("utf8"));
      } catch {
        parsedBody = undefined;
      }
    }
    const summary = requestSummary(requestId, path.split("?", 1)[0], parsedBody);
    if (summary) emit(summary);

    const upstreamHeaders = { ...clientReq.headers };
    delete upstreamHeaders.host;
    delete upstreamHeaders.connection;

    const upstreamReq = http.request(
      {
        protocol: upstreamBase.protocol,
        hostname: upstreamBase.hostname,
        port: upstreamBase.port || 80,
        method: clientReq.method,
        path,
        headers: upstreamHeaders,
      },
      (upstreamRes) => {
        const responseHeaders = { ...upstreamRes.headers };
        delete responseHeaders.connection;
        clientRes.writeHead(upstreamRes.statusCode ?? 502, responseHeaders);
        const observer = createResponseObserver(requestId, path.split("?", 1)[0], upstreamRes.statusCode ?? 502);

        upstreamRes.on("data", (chunk) => {
          observer.observe(chunk);
          clientRes.write(chunk);
        });
        upstreamRes.on("end", () => {
          clientRes.end();
          if (path.split("?", 1)[0] === "/api/chat") emit(observer.finish());
        });
      },
    );

    upstreamReq.on("error", (error) => {
      if (!clientRes.headersSent) {
        clientRes.writeHead(502, { "content-type": "text/plain" });
      }
      clientRes.end("upstream request failed");
      emit({ requestId, direction: "proxy-error", path: path.split("?", 1)[0], errorName: error?.name ?? "Error" });
    });

    if (bodyBuffer.length > 0) upstreamReq.write(bodyBuffer);
    upstreamReq.end();
  });
});

server.on("close", () => {
  for (const fd of [diagFd, workflowDiagFd]) {
    if (fd === undefined) continue;
    try {
      closeSync(fd);
    } catch {
      // Best-effort cleanup only.
    }
  }
});

server.listen(listenPort, listenHost, () => {
  console.log(`ollama_boundary_proxy=READY address=${listenHost}:${listenPort}`);
});
