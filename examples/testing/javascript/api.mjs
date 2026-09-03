const defaultTimeoutMs = 5_000;

export function normalizeAPIBase(value) {
  return value.replace(/\/+$/, "");
}

async function withTimeout(dependencies, operation) {
  const fetchImpl = dependencies.fetchImpl || globalThis.fetch;
  const timeoutMs = dependencies.timeoutMs || defaultTimeoutMs;
  const controller = new AbortController();
  const timer = setTimeout(
    () => controller.abort(new Error(`HTTP request exceeded ${timeoutMs} ms`)),
    timeoutMs,
  );
  try {
    return await operation(fetchImpl, controller.signal);
  } finally {
    clearTimeout(timer);
  }
}

export async function requestJSON(apiBase, path, options = {}, dependencies = {}) {
  return withTimeout(dependencies, async (fetchImpl, signal) => {
    const response = await fetchImpl(`${apiBase}${path}`, { ...options, signal });
    const body = response.ok ? await response.json() : null;
    if (!response.ok && response.body) await response.body.cancel();
    return { response, body };
  });
}

export async function requestStatus(apiBase, path, options = {}, dependencies = {}) {
  return withTimeout(dependencies, async (fetchImpl, signal) => {
    const response = await fetchImpl(
      `${apiBase}${path}`,
      { ...options, redirect: "manual", signal },
    );
    if (response.body) await response.body.cancel();
    return response;
  });
}

export async function findMatchingMessages(apiBase, recipient, subject, dependencies = {}) {
  const query = new URLSearchParams({ to: recipient, limit: "10" });
  const { response, body: page } = await requestJSON(
    apiBase,
    `/api/v1/emails?${query}`,
    {},
    dependencies,
  );
  if (!response.ok) throw new Error(`list failed: ${response.status}`);
  return page.emails.filter((email) => email.subject === subject);
}

export async function cleanupCapturedEmail(
  apiBase,
  recipient,
  subject,
  knownID,
  dependencies = {},
) {
  const ids = knownID
    ? [knownID]
    : (await findMatchingMessages(apiBase, recipient, subject, dependencies)).map((email) => email.id);
  if (ids.length === 0) {
    throw new Error(`cleanup could not locate the accepted message for ${recipient}`);
  }
  for (const id of ids) {
    const response = await requestStatus(
      apiBase,
      `/api/v1/emails/${encodeURIComponent(id)}`,
      { method: "DELETE" },
      dependencies,
    );
    if (!response.ok) throw new Error(`cleanup failed: ${response.status}`);
  }
}
