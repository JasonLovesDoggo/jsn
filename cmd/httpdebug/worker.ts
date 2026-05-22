const maxBodyBytes = 64 * 1024;
const maxEntries = 20;

interface CapturedHeader {
	name: string;
	value: string;
}

interface CapturedRequest {
	at: string;
	method: string;
	path: string;
	headers: CapturedHeader[];
	body: string;
	bodyTruncated: boolean;
	key: string;
}

interface StoredBucket {
	total: number;
	entries: CapturedRequest[];
}

interface RequestStore {
	get(key: string, type: "json"): Promise<StoredBucket | null>;
	put(key: string, value: string): Promise<void>;
}

interface Env {
	REQUESTS: RequestStore;
}

function wantsHTML(request: Request): boolean {
	return request.headers.get("accept")?.includes("text/html") ?? false;
}

function escapeHTML(value: string): string {
	return value.replace(/[&<>"']/g, (char) => {
		switch (char) {
		case "&":
			return "&amp;";
		case "<":
			return "&lt;";
		case ">":
			return "&gt;";
		case "\"":
			return "&quot;";
		case "'":
			return "&#39;";
		default:
			return char;
		}
	});
}

function formatRequest(request: Request): string {
	const url = new URL(request.url);
	const lines = [`${request.method} ${url.pathname}${url.search}`];

	for (const [name, value] of request.headers) {
		lines.push(`${name}: ${value}`);
	}

	return `${lines.join("\n")}\n`;
}

function captureKey(pathname: string): string {
	const key = pathname.replace(/^\/+/, "");

	if (key === "" || key.startsWith(".jsn/") || key === "value" || key.startsWith("value/")) {
		return "";
	}

	return key;
}

function valueKey(pathname: string): string {
	if (pathname === "/value" || pathname === "/value/") {
		return "";
	}

	if (!pathname.startsWith("/value/")) {
		return "";
	}

	return pathname.slice("/value/".length);
}

function storageKey(key: string): string {
	return `capture:${key}`;
}

async function readBody(request: Request): Promise<{ body: string; bodyTruncated: boolean }> {
	if (request.method === "GET" || request.method === "HEAD" || request.body === null) {
		return {
			body: "",
			bodyTruncated: false,
		};
	}

	const reader = request.body.getReader();
	const chunks: Uint8Array[] = [];
	let size = 0;
	let truncated = false;

	for (;;) {
		const { done, value } = await reader.read();

		if (done) {
			break;
		}

		if (size + value.byteLength > maxBodyBytes) {
			const remaining = maxBodyBytes - size;

			if (remaining > 0) {
				chunks.push(value.slice(0, remaining));
				size += remaining;
			}

			truncated = true;
			await reader.cancel();
			break;
		}

		chunks.push(value);
		size += value.byteLength;
	}

	const bytes = new Uint8Array(size);
	let offset = 0;

	for (const chunk of chunks) {
		bytes.set(chunk, offset);
		offset += chunk.byteLength;
	}

	return {
		body: new TextDecoder().decode(bytes),
		bodyTruncated: truncated,
	};
}

async function captureRequest(request: Request, key: string): Promise<CapturedRequest> {
	const url = new URL(request.url);
	const headers = [...request.headers].map(([name, value]) => ({ name, value }));
	const { body, bodyTruncated } = await readBody(request);

	return {
		at: new Date().toISOString(),
		method: request.method,
		path: `${url.pathname}${url.search}`,
		headers,
		body,
		bodyTruncated,
		key,
	};
}

function formatCapture(record: CapturedRequest): string {
	const lines = [
		`at: ${record.at}`,
		`${record.method} ${record.path}`,
	];

	for (const header of record.headers) {
		lines.push(`${header.name}: ${header.value}`);
	}

	if (record.body !== "") {
		lines.push("", record.bodyTruncated ? `${record.body}\n[body truncated]` : record.body);
	}

	return `${lines.join("\n")}\n`;
}

function formatCaptures(key: string, bucket: StoredBucket): string {
	if (bucket.entries.length === 0) {
		return `No requests saved for ${key}\n`;
	}

	const lines = [
		`key: ${key}`,
		`total: ${bucket.total}`,
		`showing: ${bucket.entries.length}`,
		"",
	];

	for (const [index, entry] of bucket.entries.entries()) {
		lines.push(`--- request ${bucket.entries.length - index} ---`);
		lines.push(formatCapture(entry).trimEnd());
		lines.push("");
	}

	return `${lines.join("\n").trimEnd()}\n`;
}

async function loadBucket(env: Env, key: string): Promise<StoredBucket> {
	const existing = await env.REQUESTS.get(storageKey(key), "json");

	if (existing && Array.isArray(existing.entries)) {
		return existing;
	}

	return {
		total: 0,
		entries: [],
	};
}

async function saveCapture(env: Env, record: CapturedRequest): Promise<StoredBucket> {
	const bucket = await loadBucket(env, record.key);

	bucket.total += 1;
	bucket.entries.unshift(record);
	bucket.entries = bucket.entries.slice(0, maxEntries);

	await env.REQUESTS.put(storageKey(record.key), JSON.stringify(bucket));

	return bucket;
}

function textResponse(request: Request, text: string, init: ResponseInit = {}): Response {
	const headers = new Headers(init.headers);

	if (wantsHTML(request)) {
		headers.set("content-type", "text/html; charset=utf-8");

		return new Response(`<pre id="main"><code>${escapeHTML(text)}</code></pre>\n`, {
			...init,
			headers,
		});
	}

	headers.set("content-type", "text/plain; charset=utf-8");

	return new Response(text, {
		...init,
		headers,
	});
}

export default {
	async fetch(request: Request, env: Env): Promise<Response> {
		const url = new URL(request.url);

		if (url.pathname === "/.jsn/health") {
			return new Response("OK\n", {
				headers: {
					"content-type": "text/plain; charset=utf-8",
				},
			});
		}

		const keyToRead = valueKey(url.pathname);

		if (keyToRead !== "") {
			const bucket = await loadBucket(env, keyToRead);

			if (bucket.entries.length === 0) {
				return textResponse(request, `No requests saved for ${keyToRead}\n`, {
					status: 404,
				});
			}

			return textResponse(request, formatCaptures(keyToRead, bucket));
		}

		const keyToCapture = captureKey(url.pathname);

		if (keyToCapture !== "") {
			const record = await captureRequest(request, keyToCapture);
			await saveCapture(env, record);

			return textResponse(request, formatCapture(record), {
				headers: {
					"x-debug-value": `${url.origin}/value/${keyToCapture}`,
				},
			});
		}

		const text = formatRequest(request);

		return textResponse(request, text);
	},
};
