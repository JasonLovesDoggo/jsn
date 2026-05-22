const maxBodyBytes = 64 * 1024;
const maxEntries = 20;

interface CapturedHeader {
	name: string;
	value: string;
}

interface CapturedRequestSummary {
	url: string;
	host: string;
	path: string;
	query: CapturedHeader[];
	method: string;
	referrer: string;
	referrerPolicy: string;
	cfConnectingIp: string;
	xForwardedFor: string;
	xRealIp: string;
	userAgent: string;
	accept: string;
	acceptLanguage: string;
	acceptEncoding: string;
}

interface CapturedCloudflare {
	network: CapturedCloudflareNetwork;
	geo: CapturedCloudflareGeo;
	tls: CapturedCloudflareTls;
	bot: CapturedCloudflareBot | null;
	hostMetadata: unknown;
	raw: IncomingCfProperties | null;
}

interface CapturedCloudflareNetwork {
	colo?: string;
	httpProtocol?: string;
	requestPriority?: string | null;
	clientAcceptEncoding?: string | null;
	clientTcpRtt?: number;
	clientQuicRtt?: number;
	edgeDeliveryRate?: number;
}

interface CapturedCloudflareGeo {
	asn?: number;
	asOrganization?: string;
	country?: string | null;
	isEUCountry?: string | boolean | null;
	city?: string | null;
	continent?: string | null;
	region?: string | null;
	regionCode?: string | null;
	timezone?: string;
	latitude?: string | null;
	longitude?: string | null;
	postalCode?: string | null;
	metroCode?: string | null;
}

interface CapturedCloudflareTls {
	tlsVersion?: string;
	tlsCipher?: string;
	tlsClientAuth: unknown;
	tlsClientCiphersSha1?: string;
	tlsClientExtensionsSha1?: string;
	tlsClientExtensionsSha1Le?: string;
	tlsClientHelloLength?: string;
	tlsClientRandom?: string;
}

interface CapturedCloudflareBot {
	score?: number;
	verifiedBot?: boolean;
	signedAgent?: boolean;
	staticResource?: boolean;
	ja3Hash?: string;
	ja4?: string;
	detectionIds?: number[];
	ja4Signals?: unknown;
	jaSignalsParsed?: unknown;
}

interface IncomingCfProperties {
	asn?: number;
	asOrganization?: string;
	botManagement?: CapturedCloudflareBot | null;
	clientAcceptEncoding?: string | null;
	clientQuicRtt?: number;
	clientTcpRtt?: number;
	colo?: string;
	country?: string | null;
	edgeL4?: {
		deliveryRate?: number;
	};
	hostMetadata?: unknown;
	httpProtocol?: string;
	isEUCountry?: string | boolean | null;
	requestPriority?: string | null;
	tlsCipher?: string;
	tlsClientAuth?: unknown;
	tlsClientCiphersSha1?: string;
	tlsClientExtensionsSha1?: string;
	tlsClientExtensionsSha1Le?: string;
	tlsClientHelloLength?: string;
	tlsClientRandom?: string;
	tlsVersion?: string;
	city?: string | null;
	continent?: string | null;
	latitude?: string | null;
	longitude?: string | null;
	postalCode?: string | null;
	metroCode?: string | null;
	region?: string | null;
	regionCode?: string | null;
	timezone?: string;
}

interface CapturableRequest extends Request {
	cf?: IncomingCfProperties;
}

interface CapturedRequest {
	at: string;
	method: string;
	path: string;
	request?: CapturedRequestSummary;
	cloudflare?: CapturedCloudflare;
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

function wantsHTML(request: CapturableRequest): boolean {
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

function formatRequest(request: CapturableRequest): string {
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

async function readBody(request: CapturableRequest): Promise<{ body: string; bodyTruncated: boolean }> {
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

function headerValue(request: CapturableRequest, name: string): string {
	return request.headers.get(name) ?? "";
}

function capturedHeaderValue(headers: CapturedHeader[], name: string): string {
	const found = headers.find((header) => header.name.toLowerCase() === name.toLowerCase());

	return found?.value ?? "";
}

function captureRequestSummary(request: CapturableRequest): CapturedRequestSummary {
	const url = new URL(request.url);
	const query = [...url.searchParams].map(([name, value]) => ({ name, value }));

	return {
		url: request.url,
		host: url.host,
		path: `${url.pathname}${url.search}`,
		query,
		method: request.method,
		referrer: request.referrer,
		referrerPolicy: request.referrerPolicy,
		cfConnectingIp: headerValue(request, "cf-connecting-ip"),
		xForwardedFor: headerValue(request, "x-forwarded-for"),
		xRealIp: headerValue(request, "x-real-ip"),
		userAgent: headerValue(request, "user-agent"),
		accept: headerValue(request, "accept"),
		acceptLanguage: headerValue(request, "accept-language"),
		acceptEncoding: headerValue(request, "accept-encoding"),
	};
}

function emptyCloudflare(): CapturedCloudflare {
	return {
		network: {},
		geo: {},
		tls: {
			tlsClientAuth: null,
		},
		bot: null,
		hostMetadata: null,
		raw: null,
	};
}

function captureCloudflare(cf: IncomingCfProperties | undefined): CapturedCloudflare {
	if (cf === undefined) {
		return emptyCloudflare();
	}

	return {
		network: {
			colo: cf.colo,
			httpProtocol: cf.httpProtocol,
			requestPriority: cf.requestPriority,
			clientAcceptEncoding: cf.clientAcceptEncoding,
			clientTcpRtt: cf.clientTcpRtt,
			clientQuicRtt: cf.clientQuicRtt,
			edgeDeliveryRate: cf.edgeL4?.deliveryRate,
		},
		geo: {
			asn: cf.asn,
			asOrganization: cf.asOrganization,
			country: cf.country,
			isEUCountry: cf.isEUCountry,
			city: cf.city,
			continent: cf.continent,
			region: cf.region,
			regionCode: cf.regionCode,
			timezone: cf.timezone,
			latitude: cf.latitude,
			longitude: cf.longitude,
			postalCode: cf.postalCode,
			metroCode: cf.metroCode,
		},
		tls: {
			tlsVersion: cf.tlsVersion,
			tlsCipher: cf.tlsCipher,
			tlsClientAuth: cf.tlsClientAuth ?? null,
			tlsClientCiphersSha1: cf.tlsClientCiphersSha1,
			tlsClientExtensionsSha1: cf.tlsClientExtensionsSha1,
			tlsClientExtensionsSha1Le: cf.tlsClientExtensionsSha1Le,
			tlsClientHelloLength: cf.tlsClientHelloLength,
			tlsClientRandom: cf.tlsClientRandom,
		},
		bot: cf.botManagement ?? null,
		hostMetadata: cf.hostMetadata ?? null,
		raw: cf,
	};
}

function requestSummary(record: CapturedRequest): CapturedRequestSummary {
	if (record.request !== undefined) {
		return record.request;
	}

	return {
		url: "",
		host: capturedHeaderValue(record.headers, "host"),
		path: record.path,
		query: [],
		method: record.method,
		referrer: capturedHeaderValue(record.headers, "referer"),
		referrerPolicy: "",
		cfConnectingIp: capturedHeaderValue(record.headers, "cf-connecting-ip"),
		xForwardedFor: capturedHeaderValue(record.headers, "x-forwarded-for"),
		xRealIp: capturedHeaderValue(record.headers, "x-real-ip"),
		userAgent: capturedHeaderValue(record.headers, "user-agent"),
		accept: capturedHeaderValue(record.headers, "accept"),
		acceptLanguage: capturedHeaderValue(record.headers, "accept-language"),
		acceptEncoding: capturedHeaderValue(record.headers, "accept-encoding"),
	};
}

function cloudflareSummary(record: CapturedRequest): CapturedCloudflare {
	return record.cloudflare ?? emptyCloudflare();
}

async function captureRequest(request: CapturableRequest, key: string): Promise<CapturedRequest> {
	const url = new URL(request.url);
	const headers = [...request.headers].map(([name, value]) => ({ name, value }));
	const { body, bodyTruncated } = await readBody(request);

	return {
		at: new Date().toISOString(),
		method: request.method,
		path: `${url.pathname}${url.search}`,
		request: captureRequestSummary(request),
		cloudflare: captureCloudflare(request.cf),
		headers,
		body,
		bodyTruncated,
		key,
	};
}

function formatPair(label: string, value: unknown): string | null {
	if (value === undefined || value === null || value === "") {
		return null;
	}

	if (typeof value === "object") {
		return `${label}: ${JSON.stringify(value)}`;
	}

	return `${label}: ${String(value)}`;
}

function pushSection(lines: string[], title: string, pairs: Array<[string, unknown]>): void {
	const section = pairs
		.map(([label, value]) => formatPair(label, value))
		.filter((line): line is string => line !== null);

	if (section.length === 0) {
		return;
	}

	lines.push("", `${title}:`, ...section.map((line) => `  ${line}`));
}

function formatCapture(record: CapturedRequest): string {
	const client = requestSummary(record);
	const cloudflare = cloudflareSummary(record);
	const lines = [
		`at: ${record.at}`,
		`${record.method} ${record.path}`,
	];

	pushSection(lines, "client", [
		["url", client.url],
		["host", client.host],
		["query", client.query],
		["ip", client.cfConnectingIp],
		["x-forwarded-for", client.xForwardedFor],
		["x-real-ip", client.xRealIp],
		["user-agent", client.userAgent],
		["accept", client.accept],
		["accept-language", client.acceptLanguage],
		["accept-encoding", client.acceptEncoding],
		["referrer", client.referrer],
		["referrer-policy", client.referrerPolicy],
	]);

	pushSection(lines, "cloudflare network", [
		["colo", cloudflare.network.colo],
		["httpProtocol", cloudflare.network.httpProtocol],
		["requestPriority", cloudflare.network.requestPriority],
		["clientAcceptEncoding", cloudflare.network.clientAcceptEncoding],
		["clientTcpRtt", cloudflare.network.clientTcpRtt],
		["clientQuicRtt", cloudflare.network.clientQuicRtt],
		["edgeDeliveryRate", cloudflare.network.edgeDeliveryRate],
	]);

	pushSection(lines, "cloudflare geo", [
		["asn", cloudflare.geo.asn],
		["asOrganization", cloudflare.geo.asOrganization],
		["country", cloudflare.geo.country],
		["isEUCountry", cloudflare.geo.isEUCountry],
		["city", cloudflare.geo.city],
		["continent", cloudflare.geo.continent],
		["region", cloudflare.geo.region],
		["regionCode", cloudflare.geo.regionCode],
		["timezone", cloudflare.geo.timezone],
		["latitude", cloudflare.geo.latitude],
		["longitude", cloudflare.geo.longitude],
		["postalCode", cloudflare.geo.postalCode],
		["metroCode", cloudflare.geo.metroCode],
	]);

	pushSection(lines, "cloudflare tls", [
		["tlsVersion", cloudflare.tls.tlsVersion],
		["tlsCipher", cloudflare.tls.tlsCipher],
		["tlsClientCiphersSha1", cloudflare.tls.tlsClientCiphersSha1],
		["tlsClientExtensionsSha1", cloudflare.tls.tlsClientExtensionsSha1],
		["tlsClientExtensionsSha1Le", cloudflare.tls.tlsClientExtensionsSha1Le],
		["tlsClientHelloLength", cloudflare.tls.tlsClientHelloLength],
		["tlsClientRandom", cloudflare.tls.tlsClientRandom],
		["tlsClientAuth", cloudflare.tls.tlsClientAuth],
	]);

	if (cloudflare.bot !== null) {
		pushSection(lines, "cloudflare bot", [
			["score", cloudflare.bot.score],
			["verifiedBot", cloudflare.bot.verifiedBot],
			["signedAgent", cloudflare.bot.signedAgent],
			["staticResource", cloudflare.bot.staticResource],
			["ja3Hash", cloudflare.bot.ja3Hash],
			["ja4", cloudflare.bot.ja4],
			["detectionIds", cloudflare.bot.detectionIds],
			["ja4Signals", cloudflare.bot.ja4Signals],
			["jaSignalsParsed", cloudflare.bot.jaSignalsParsed],
		]);
	}

	pushSection(lines, "cloudflare misc", [
		["hostMetadata", cloudflare.hostMetadata],
		["raw", cloudflare.raw],
	]);

	lines.push("", "headers:");

	for (const header of record.headers) {
		lines.push(`  ${header.name}: ${header.value}`);
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

function textResponse(request: CapturableRequest, text: string, init: ResponseInit = {}): Response {
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
	async fetch(request: CapturableRequest, env: Env): Promise<Response> {
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
