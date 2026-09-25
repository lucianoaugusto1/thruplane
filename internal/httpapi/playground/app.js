"use strict";

(() => {
  const MAX_FILE_BYTES = 4 * 1024 * 1024;
  const state = {
    messages: [],
    attachments: [],
    controller: null,
    modelDetail: null,
    lastPayload: null,
  };

  const byID = (id) => document.getElementById(id);
  const elements = {
    apiKey: byID("api-key"),
    connect: byID("connect-button"),
    connectionState: byID("connection-state"),
    model: byID("model"),
    modelSummary: byID("model-summary"),
    systemPrompt: byID("system-prompt"),
    temperature: byID("temperature"),
    temperatureValue: byID("temperature-value"),
    maxTokens: byID("max-tokens"),
    stream: byID("stream"),
    tools: byID("tools"),
    files: byID("files"),
    attachments: byID("attachments"),
    prompt: byID("prompt"),
    send: byID("send-button"),
    stop: byID("stop-button"),
    clear: byID("clear-button"),
    exportCurl: byID("export-curl"),
    conversation: byID("conversation"),
    emptyState: byID("empty-state"),
    liveStatus: byID("live-status"),
    requestJSON: byID("request-json"),
    rawResponse: byID("raw-response"),
    routeProvider: byID("route-provider"),
    routeModel: byID("route-model"),
    routeAttempts: byID("route-attempts"),
    routeFallbacks: byID("route-fallbacks"),
    requestID: byID("request-id"),
    upstreamRequestID: byID("upstream-request-id"),
    responseStatus: byID("response-status"),
    ttft: byID("ttft"),
    totalLatency: byID("total-latency"),
    usage: byID("usage"),
  };

  function authHeaders(includeJSON = false) {
    const headers = {};
    const key = elements.apiKey.value.trim();
    if (key) {
      headers.Authorization = `Bearer ${key}`;
    }
    if (includeJSON) {
      headers["Content-Type"] = "application/json";
    }
    return headers;
  }

  async function loadModels() {
    setConnectionState("Connecting…", "pending");
    try {
      const response = await fetch("/v1/models", { headers: authHeaders() });
      if (!response.ok) {
        throw new Error(await readError(response));
      }
      const payload = await response.json();
      const models = Array.isArray(payload.data) ? payload.data : [];
      elements.model.replaceChildren();
      for (const model of models) {
        const option = document.createElement("option");
        option.value = String(model.id || "");
        option.textContent = String(model.id || "Unnamed model");
        elements.model.append(option);
      }
      if (models.length === 0) {
        throw new Error("The gateway returned no configured model aliases.");
      }
      elements.model.disabled = false;
      elements.send.disabled = false;
      setConnectionState(`${models.length} model${models.length === 1 ? "" : "s"} available`, "ready");
      await loadModelDetail();
    } catch (error) {
      elements.model.replaceChildren();
      elements.model.disabled = true;
      elements.send.disabled = true;
      setConnectionState(error.message, "error");
    }
  }

  async function loadModelDetail() {
    const alias = elements.model.value;
    if (!alias) {
      return;
    }
    try {
      const response = await fetch(`/v1/models/${encodeURIComponent(alias)}`, {
        headers: authHeaders(),
      });
      if (!response.ok) {
        throw new Error(await readError(response));
      }
      state.modelDetail = await response.json();
      renderModelDetail(state.modelDetail);
    } catch (error) {
      state.modelDetail = null;
      elements.modelSummary.replaceChildren();
      appendText(elements.modelSummary, error.message, "model-error");
    }
  }

  function renderModelDetail(detail) {
    elements.modelSummary.replaceChildren();
    const targets = Array.isArray(detail.targets) ? detail.targets : [];
    for (const target of targets) {
      const card = document.createElement("article");
      card.className = "target-card";
      const title = document.createElement("strong");
      title.textContent = `${target.provider || "provider"} / ${target.model || "model"}`;
      card.append(title);

      const capabilities = target.effective_capabilities;
      if (capabilities) {
        const facts = [];
        if (Array.isArray(capabilities.input_modalities)) {
          facts.push(capabilities.input_modalities.join(" · "));
        }
        if (capabilities.streaming) {
          facts.push("streaming");
        }
        if (capabilities.tools && capabilities.tools.function_calling) {
          facts.push("tools");
        }
        if (facts.length > 0) {
          appendText(card, facts.join(" · "), "target-facts");
        }
      } else {
        appendText(card, "Uncataloged target", "target-facts");
      }
      elements.modelSummary.append(card);
    }
  }

  function parseTools() {
    const source = elements.tools.value.trim();
    if (!source) {
      return undefined;
    }
    let tools;
    try {
      tools = JSON.parse(source);
    } catch (error) {
      throw new Error(`Tools JSON is invalid: ${error.message}`);
    }
    if (!Array.isArray(tools)) {
      throw new Error("Tools JSON must be an array.");
    }
    return tools;
  }

  function buildPayload(includeDraft = true) {
    const model = elements.model.value;
    if (!model) {
      throw new Error("Connect to the gateway and select a model first.");
    }
    const messages = [];
    const system = elements.systemPrompt.value.trim();
    if (system) {
      messages.push({ role: "system", content: system });
    }
    messages.push(...state.messages);

    if (includeDraft) {
      const draft = buildDraftMessage();
      if (draft) {
        messages.push(draft);
      }
    }
    if (messages.length === 0) {
      throw new Error("Enter a message or attach a supported file.");
    }

    const payload = { model, messages, stream: elements.stream.checked };
    const temperature = Number(elements.temperature.value);
    if (Number.isFinite(temperature)) {
      payload.temperature = temperature;
    }
    const maxTokens = Number.parseInt(elements.maxTokens.value, 10);
    if (Number.isInteger(maxTokens) && maxTokens > 0) {
      payload.max_completion_tokens = maxTokens;
    }
    const tools = parseTools();
    if (tools && tools.length > 0) {
      payload.tools = tools;
    }
    return payload;
  }

  function buildDraftMessage() {
    const text = elements.prompt.value.trim();
    if (!text && state.attachments.length === 0) {
      return null;
    }
    if (state.attachments.length === 0) {
      return { role: "user", content: text };
    }
    const content = [];
    if (text) {
      content.push({ type: "text", text });
    }
    for (const attachment of state.attachments) {
      content.push(attachment.part);
    }
    return { role: "user", content };
  }

  async function sendMessage() {
    if (state.controller) {
      return;
    }
    let payload;
    let draft;
    try {
      draft = buildDraftMessage();
      payload = buildPayload(true);
    } catch (error) {
      announce(error.message, "error");
      return;
    }

    if (draft) {
      state.messages.push(draft);
      appendMessage(draft);
      elements.prompt.value = "";
      state.attachments = [];
      renderAttachments();
    }
    state.lastPayload = payload;
    elements.requestJSON.textContent = formatJSON(payload);
    elements.rawResponse.textContent = "Waiting for response…";
    resetMetrics();
    setBusy(true);

    const startedAt = performance.now();
    const controller = new AbortController();
    state.controller = controller;
    try {
      const response = await fetch("/v1/chat/completions", {
        method: "POST",
        headers: authHeaders(true),
        body: JSON.stringify(payload),
        signal: controller.signal,
      });
      applyRouteHeaders(response);
      if (!response.ok) {
        throw new Error(await readError(response));
      }

      let assistant;
      if (payload.stream) {
        assistant = await consumeSSE(response, startedAt);
      } else {
        const body = await response.json();
        elements.rawResponse.textContent = formatJSON(body);
        assistant = body.choices && body.choices[0] ? body.choices[0].message : null;
        renderUsage(body.usage);
      }
      if (assistant) {
        state.messages.push(assistant);
        appendMessage(assistant);
      }
      elements.totalLatency.textContent = formatDuration(performance.now() - startedAt);
      announce("Response completed.", "ready");
    } catch (error) {
      if (error.name === "AbortError") {
        announce("Request canceled.", "pending");
      } else {
        announce(error.message, "error");
        appendNotice(error.message);
      }
      elements.totalLatency.textContent = formatDuration(performance.now() - startedAt);
    } finally {
      state.controller = null;
      setBusy(false);
    }
  }

  async function consumeSSE(response, startedAt) {
    if (!response.body) {
      throw new Error("The browser did not expose a streaming response body.");
    }
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    let firstByte = true;
    let content = "";
    let usage;
    const toolCalls = new Map();
    const liveView = appendMessage({ role: "assistant", content: "" }, true);

    while (true) {
      const { value, done } = await reader.read();
      if (done) {
        break;
      }
      if (firstByte) {
        firstByte = false;
        elements.ttft.textContent = formatDuration(performance.now() - startedAt);
      }
      buffer += decoder.decode(value, { stream: true });
      const events = buffer.split(/\r?\n\r?\n/);
      buffer = events.pop() || "";
      for (const event of events) {
        const data = event
          .split(/\r?\n/)
          .filter((line) => line.startsWith("data:"))
          .map((line) => line.slice(5).trimStart())
          .join("\n");
        if (!data || data === "[DONE]") {
          continue;
        }
        let chunk;
        try {
          chunk = JSON.parse(data);
        } catch (_error) {
          continue;
        }
        elements.rawResponse.textContent = formatJSON(chunk);
        if (chunk.usage) {
          usage = chunk.usage;
        }
        const delta = chunk.choices && chunk.choices[0] && chunk.choices[0].delta;
        if (!delta) {
          continue;
        }
        if (typeof delta.content === "string") {
          content += delta.content;
        }
        mergeToolCalls(toolCalls, delta.tool_calls);
        updateMessageView(liveView, content, [...toolCalls.values()]);
      }
    }

    liveView.container.remove();
    renderUsage(usage);
    const assistant = { role: "assistant", content: content || null };
    if (toolCalls.size > 0) {
      assistant.tool_calls = [...toolCalls.values()];
    }
    return assistant;
  }

  function mergeToolCalls(target, chunks) {
    if (!Array.isArray(chunks)) {
      return;
    }
    for (const chunk of chunks) {
      const index = Number.isInteger(chunk.index) ? chunk.index : target.size;
      const existing = target.get(index) || {
        id: "",
        type: "function",
        function: { name: "", arguments: "" },
      };
      if (chunk.id) {
        existing.id = chunk.id;
      }
      if (chunk.type) {
        existing.type = chunk.type;
      }
      if (chunk.function) {
        existing.function.name += chunk.function.name || "";
        existing.function.arguments += chunk.function.arguments || "";
      }
      target.set(index, existing);
    }
  }

  async function handleFiles(event) {
    const files = [...event.target.files];
    event.target.value = "";
    for (const file of files) {
      try {
        if (file.size > MAX_FILE_BYTES) {
          throw new Error(`${file.name} exceeds the 4 MiB playground limit.`);
        }
        const part = await fileToContentPart(file);
        state.attachments.push({ name: file.name, size: file.size, part });
      } catch (error) {
        announce(error.message, "error");
      }
    }
    renderAttachments();
  }

  async function fileToContentPart(file) {
    const dataURL = await readDataURL(file);
    if (["image/png", "image/jpeg", "image/gif", "image/webp"].includes(file.type)) {
      return { type: "image_url", image_url: { url: dataURL } };
    }
    if (file.type === "application/pdf") {
      return { type: "file", file: { file_data: dataURL } };
    }
    const audioFormats = {
      "audio/wav": "wav",
      "audio/x-wav": "wav",
      "audio/mpeg": "mp3",
      "audio/mp3": "mp3",
    };
    const format = audioFormats[file.type];
    if (format) {
      return {
        type: "input_audio",
        input_audio: { data: dataURL.split(",", 2)[1], format },
      };
    }
    throw new Error(`${file.name} is not a supported image, PDF, WAV, or MP3 file.`);
  }

  function readDataURL(file) {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.addEventListener("load", () => resolve(String(reader.result)));
      reader.addEventListener("error", () => reject(new Error(`Could not read ${file.name}.`)));
      reader.readAsDataURL(file);
    });
  }

  function renderAttachments() {
    elements.attachments.replaceChildren();
    state.attachments.forEach((attachment, index) => {
      const chip = document.createElement("span");
      chip.className = "attachment-chip";
      const label = document.createElement("span");
      label.textContent = `${attachment.name} · ${formatBytes(attachment.size)}`;
      const remove = document.createElement("button");
      remove.type = "button";
      remove.className = "chip-remove";
      remove.setAttribute("aria-label", `Remove ${attachment.name}`);
      remove.textContent = "×";
      remove.addEventListener("click", () => {
        state.attachments.splice(index, 1);
        renderAttachments();
      });
      chip.append(label, remove);
      elements.attachments.append(chip);
    });
  }

  function appendMessage(message, live = false) {
    elements.emptyState.hidden = true;
    const container = document.createElement("article");
    container.className = `message message-${message.role || "assistant"}`;
    if (live) {
      container.classList.add("message-live");
    }
    const header = document.createElement("header");
    header.textContent = roleLabel(message.role);
    const body = document.createElement("div");
    body.className = "message-body";
    const tools = document.createElement("pre");
    tools.className = "tool-calls";
    tools.hidden = true;
    container.append(header, body, tools);
    elements.conversation.append(container);
    updateMessageView({ container, body, tools }, messageText(message), message.tool_calls || []);
    elements.conversation.scrollTop = elements.conversation.scrollHeight;
    return { container, body, tools };
  }

  function updateMessageView(view, content, toolCalls) {
    view.body.textContent = content || (toolCalls.length ? "Tool call requested" : "Waiting…");
    if (toolCalls.length > 0) {
      view.tools.hidden = false;
      view.tools.textContent = formatJSON(toolCalls);
    } else {
      view.tools.hidden = true;
      view.tools.textContent = "";
    }
    elements.conversation.scrollTop = elements.conversation.scrollHeight;
  }

  function messageText(message) {
    if (typeof message.content === "string") {
      return message.content;
    }
    if (!Array.isArray(message.content)) {
      return "";
    }
    return message.content.map((part) => {
      if (part.type === "text" || part.type === "input_text") {
        return part.text || "";
      }
      if (part.type === "image_url") {
        return "[Image attached]";
      }
      if (part.type === "file") {
        return "[PDF attached]";
      }
      if (part.type === "input_audio") {
        return "[Audio attached]";
      }
      return "[Attachment]";
    }).filter(Boolean).join("\n");
  }

  function appendNotice(message) {
    const notice = document.createElement("div");
    notice.className = "conversation-notice";
    notice.textContent = message;
    elements.conversation.append(notice);
  }

  function applyRouteHeaders(response) {
    const values = [
      [elements.routeProvider, "X-NexoRoute-Provider"],
      [elements.routeModel, "X-NexoRoute-Model"],
      [elements.routeAttempts, "X-NexoRoute-Attempts"],
      [elements.routeFallbacks, "X-NexoRoute-Fallbacks"],
      [elements.requestID, "X-Request-Id"],
      [elements.upstreamRequestID, "X-Upstream-Request-Id"],
    ];
    for (const [element, header] of values) {
      element.textContent = response.headers.get(header) || "—";
    }
    elements.responseStatus.textContent = `${response.status} ${response.statusText}`.trim();
  }

  function renderUsage(usage) {
    if (!usage) {
      elements.usage.textContent = "—";
      return;
    }
    const input = usage.prompt_tokens ?? usage.input_tokens ?? 0;
    const output = usage.completion_tokens ?? usage.output_tokens ?? 0;
    const total = usage.total_tokens ?? input + output;
    elements.usage.textContent = `${input} in · ${output} out · ${total} total`;
  }

  function resetMetrics() {
    for (const element of [
      elements.routeProvider,
      elements.routeModel,
      elements.routeAttempts,
      elements.routeFallbacks,
      elements.requestID,
      elements.upstreamRequestID,
      elements.responseStatus,
      elements.ttft,
      elements.totalLatency,
      elements.usage,
    ]) {
      element.textContent = "—";
    }
  }

  async function exportCurl() {
    let payload;
    try {
      payload = state.lastPayload || buildPayload(true);
    } catch (error) {
      announce(error.message, "error");
      return;
    }
    const body = JSON.stringify(payload, null, 2).replaceAll("'", "'\"'\"'");
    const command = [
      `curl ${window.location.origin}/v1/chat/completions \\`,
      "  -H 'Content-Type: application/json' \\",
      "  -H 'Authorization: Bearer ${NEXOROUTE_API_KEY}' \\",
      `  --data-binary '${body}'`,
    ].join("\n");
    try {
      await navigator.clipboard.writeText(command);
      announce("Copied curl command with $NEXOROUTE_API_KEY placeholder.", "ready");
    } catch (_error) {
      announce("Clipboard access was denied by the browser.", "error");
    }
  }

  async function readError(response) {
    const text = await response.text();
    elements.rawResponse.textContent = text || `${response.status} ${response.statusText}`;
    try {
      const payload = JSON.parse(text);
      return payload.error && payload.error.message ? payload.error.message : text;
    } catch (_error) {
      return text || `${response.status} ${response.statusText}`;
    }
  }

  function setBusy(busy) {
    elements.send.disabled = busy || !elements.model.value;
    elements.stop.disabled = !busy;
    elements.clear.disabled = busy;
    elements.connect.disabled = busy;
    elements.model.disabled = busy || elements.model.options.length === 0;
  }

  function clearConversation() {
    state.messages = [];
    state.attachments = [];
    state.lastPayload = null;
    elements.conversation.querySelectorAll(".message, .conversation-notice").forEach((node) => node.remove());
    elements.emptyState.hidden = false;
    elements.prompt.value = "";
    elements.requestJSON.textContent = "No request yet.";
    elements.rawResponse.textContent = "No response yet.";
    renderAttachments();
    resetMetrics();
    announce("Conversation cleared.", "ready");
  }

  function setConnectionState(message, stateName) {
    elements.connectionState.textContent = message;
    elements.connectionState.dataset.state = stateName;
    announce(message, stateName);
  }

  function announce(message, stateName = "ready") {
    elements.liveStatus.textContent = message;
    elements.liveStatus.dataset.state = stateName;
  }

  function appendText(parent, text, className) {
    const node = document.createElement("span");
    node.className = className;
    node.textContent = text;
    parent.append(node);
  }

  function roleLabel(role) {
    return { user: "You", assistant: "Model", system: "System", tool: "Tool" }[role] || "Message";
  }

  function formatJSON(value) {
    return JSON.stringify(value, null, 2);
  }

  function formatDuration(milliseconds) {
    if (!Number.isFinite(milliseconds)) {
      return "—";
    }
    return milliseconds < 1000 ? `${milliseconds.toFixed(1)} ms` : `${(milliseconds / 1000).toFixed(2)} s`;
  }

  function formatBytes(bytes) {
    return bytes < 1024 * 1024 ? `${Math.ceil(bytes / 1024)} KiB` : `${(bytes / 1024 / 1024).toFixed(1)} MiB`;
  }

  elements.connect.addEventListener("click", loadModels);
  elements.model.addEventListener("change", loadModelDetail);
  elements.temperature.addEventListener("input", () => {
    elements.temperatureValue.textContent = Number(elements.temperature.value).toFixed(1);
  });
  elements.files.addEventListener("change", handleFiles);
  elements.send.addEventListener("click", sendMessage);
  elements.stop.addEventListener("click", () => state.controller && state.controller.abort());
  elements.clear.addEventListener("click", clearConversation);
  elements.exportCurl.addEventListener("click", exportCurl);
  elements.prompt.addEventListener("keydown", (event) => {
    if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
      event.preventDefault();
      sendMessage();
    }
  });

  resetMetrics();
  if (!elements.apiKey.value) {
    loadModels();
  }
})();
