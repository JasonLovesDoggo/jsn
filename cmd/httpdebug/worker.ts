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

interface CapturedBrowserHints {
	secChUa: string;
	secChUaArch: string;
	secChUaBitness: string;
	secChUaFullVersion: string;
	secChUaFullVersionList: string;
	secChUaMobile: string;
	secChUaModel: string;
	secChUaPlatform: string;
	secChUaPlatformVersion: string;
	dnt: string;
	secGpc: string;
}

interface CapturedFetchMetadata {
	secFetchDest: string;
	secFetchMode: string;
	secFetchSite: string;
	secFetchUser: string;
	origin: string;
	purpose: string;
	priority: string;
	upgradeInsecureRequests: string;
}

interface CapturedProxySignals {
	cfRay: string;
	cfVisitor: string;
	cfIpcountry: string;
	cdnLoop: string;
	forwarded: string;
	via: string;
	xForwardedHost: string;
	xForwardedPort: string;
	xForwardedProto: string;
	trueClientIp: string;
}

interface CapturedContentSummary {
	contentType: string;
	contentLength: string;
	contentEncoding: string;
	contentLanguage: string;
	authorizationPresent: boolean;
	cookiePresent: boolean;
	cookieCount: number;
	cookieNames: string[];
	bodyBytes: number;
	bodySha256: string;
}

interface CapturedDerived {
	requestLine: string;
	headerOrder: string[];
	headerNames: string[];
	headerCount: number;
	requestLineSha256: string;
	headerOrderSha256: string;
	headerNamesSha256: string;
	headersSha256: string;
	userAgentSha256: string;
	clientHintsSha256: string;
	fetchMetadataSha256: string;
	proxySignalsSha256: string;
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
	browserHints?: CapturedBrowserHints;
	fetchMetadata?: CapturedFetchMetadata;
	proxySignals?: CapturedProxySignals;
	content?: CapturedContentSummary;
	derived?: CapturedDerived;
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

async function sha256Hex(value: string): Promise<string> {
	const bytes = new TextEncoder().encode(value);
	const hash = await crypto.subtle.digest("SHA-256", bytes);
	const hashBytes = new Uint8Array(hash);

	return [...hashBytes]
		.map((byte) => byte.toString(16).padStart(2, "0"))
		.join("");
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

function captureBrowserHints(request: CapturableRequest): CapturedBrowserHints {
	return {
		secChUa: headerValue(request, "sec-ch-ua"),
		secChUaArch: headerValue(request, "sec-ch-ua-arch"),
		secChUaBitness: headerValue(request, "sec-ch-ua-bitness"),
		secChUaFullVersion: headerValue(request, "sec-ch-ua-full-version"),
		secChUaFullVersionList: headerValue(request, "sec-ch-ua-full-version-list"),
		secChUaMobile: headerValue(request, "sec-ch-ua-mobile"),
		secChUaModel: headerValue(request, "sec-ch-ua-model"),
		secChUaPlatform: headerValue(request, "sec-ch-ua-platform"),
		secChUaPlatformVersion: headerValue(request, "sec-ch-ua-platform-version"),
		dnt: headerValue(request, "dnt"),
		secGpc: headerValue(request, "sec-gpc"),
	};
}

function captureFetchMetadata(request: CapturableRequest): CapturedFetchMetadata {
	return {
		secFetchDest: headerValue(request, "sec-fetch-dest"),
		secFetchMode: headerValue(request, "sec-fetch-mode"),
		secFetchSite: headerValue(request, "sec-fetch-site"),
		secFetchUser: headerValue(request, "sec-fetch-user"),
		origin: headerValue(request, "origin"),
		purpose: headerValue(request, "purpose"),
		priority: headerValue(request, "priority"),
		upgradeInsecureRequests: headerValue(request, "upgrade-insecure-requests"),
	};
}

function captureProxySignals(request: CapturableRequest): CapturedProxySignals {
	return {
		cfRay: headerValue(request, "cf-ray"),
		cfVisitor: headerValue(request, "cf-visitor"),
		cfIpcountry: headerValue(request, "cf-ipcountry"),
		cdnLoop: headerValue(request, "cdn-loop"),
		forwarded: headerValue(request, "forwarded"),
		via: headerValue(request, "via"),
		xForwardedHost: headerValue(request, "x-forwarded-host"),
		xForwardedPort: headerValue(request, "x-forwarded-port"),
		xForwardedProto: headerValue(request, "x-forwarded-proto"),
		trueClientIp: headerValue(request, "true-client-ip"),
	};
}

function cookieNames(request: CapturableRequest): string[] {
	const cookieHeader = headerValue(request, "cookie");

	if (cookieHeader === "") {
		return [];
	}

	return cookieHeader
		.split(";")
		.map((cookie) => cookie.trim().split("=")[0]?.trim() ?? "")
		.filter((name) => name !== "");
}

async function captureContentSummary(
	request: CapturableRequest,
	body: string,
): Promise<CapturedContentSummary> {
	const cookies = cookieNames(request);

	return {
		contentType: headerValue(request, "content-type"),
		contentLength: headerValue(request, "content-length"),
		contentEncoding: headerValue(request, "content-encoding"),
		contentLanguage: headerValue(request, "content-language"),
		authorizationPresent: headerValue(request, "authorization") !== "",
		cookiePresent: cookies.length > 0,
		cookieCount: cookies.length,
		cookieNames: cookies,
		bodyBytes: new TextEncoder().encode(body).byteLength,
		bodySha256: await sha256Hex(body),
	};
}

async function captureDerived(
	request: CapturableRequest,
	headers: CapturedHeader[],
	browserHints: CapturedBrowserHints,
	fetchMetadata: CapturedFetchMetadata,
	proxySignals: CapturedProxySignals,
): Promise<CapturedDerived> {
	const url = new URL(request.url);
	const requestLine = `${request.method} ${url.pathname}${url.search}`;
	const headerOrder = headers.map((header) => header.name);
	const headerNames = [...new Set(headerOrder.map((name) => name.toLowerCase()))].sort();
	const renderedHeaders = headers.map((header) => `${header.name}: ${header.value}`).join("\n");
	const renderedClientHints = JSON.stringify(browserHints);
	const renderedFetchMetadata = JSON.stringify(fetchMetadata);
	const renderedProxySignals = JSON.stringify(proxySignals);

	return {
		requestLine,
		headerOrder,
		headerNames,
		headerCount: headers.length,
		requestLineSha256: await sha256Hex(requestLine),
		headerOrderSha256: await sha256Hex(headerOrder.join("\n")),
		headerNamesSha256: await sha256Hex(headerNames.join("\n")),
		headersSha256: await sha256Hex(renderedHeaders),
		userAgentSha256: await sha256Hex(headerValue(request, "user-agent")),
		clientHintsSha256: await sha256Hex(renderedClientHints),
		fetchMetadataSha256: await sha256Hex(renderedFetchMetadata),
		proxySignalsSha256: await sha256Hex(renderedProxySignals),
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
	const browserHints = captureBrowserHints(request);
	const fetchMetadata = captureFetchMetadata(request);
	const proxySignals = captureProxySignals(request);
	const content = await captureContentSummary(request, body);
	const derived = await captureDerived(request, headers, browserHints, fetchMetadata, proxySignals);

	return {
		at: new Date().toISOString(),
		method: request.method,
		path: `${url.pathname}${url.search}`,
		request: captureRequestSummary(request),
		browserHints,
		fetchMetadata,
		proxySignals,
		content,
		derived,
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

	if (record.browserHints !== undefined) {
		pushSection(lines, "browser hints", [
			["secChUa", record.browserHints.secChUa],
			["secChUaArch", record.browserHints.secChUaArch],
			["secChUaBitness", record.browserHints.secChUaBitness],
			["secChUaFullVersion", record.browserHints.secChUaFullVersion],
			["secChUaFullVersionList", record.browserHints.secChUaFullVersionList],
			["secChUaMobile", record.browserHints.secChUaMobile],
			["secChUaModel", record.browserHints.secChUaModel],
			["secChUaPlatform", record.browserHints.secChUaPlatform],
			["secChUaPlatformVersion", record.browserHints.secChUaPlatformVersion],
			["dnt", record.browserHints.dnt],
			["secGpc", record.browserHints.secGpc],
		]);
	}

	if (record.fetchMetadata !== undefined) {
		pushSection(lines, "fetch metadata", [
			["secFetchDest", record.fetchMetadata.secFetchDest],
			["secFetchMode", record.fetchMetadata.secFetchMode],
			["secFetchSite", record.fetchMetadata.secFetchSite],
			["secFetchUser", record.fetchMetadata.secFetchUser],
			["origin", record.fetchMetadata.origin],
			["purpose", record.fetchMetadata.purpose],
			["priority", record.fetchMetadata.priority],
			["upgradeInsecureRequests", record.fetchMetadata.upgradeInsecureRequests],
		]);
	}

	if (record.proxySignals !== undefined) {
		pushSection(lines, "proxy signals", [
			["cfRay", record.proxySignals.cfRay],
			["cfVisitor", record.proxySignals.cfVisitor],
			["cfIpcountry", record.proxySignals.cfIpcountry],
			["cdnLoop", record.proxySignals.cdnLoop],
			["forwarded", record.proxySignals.forwarded],
			["via", record.proxySignals.via],
			["xForwardedHost", record.proxySignals.xForwardedHost],
			["xForwardedPort", record.proxySignals.xForwardedPort],
			["xForwardedProto", record.proxySignals.xForwardedProto],
			["trueClientIp", record.proxySignals.trueClientIp],
		]);
	}

	if (record.content !== undefined) {
		pushSection(lines, "content", [
			["contentType", record.content.contentType],
			["contentLength", record.content.contentLength],
			["contentEncoding", record.content.contentEncoding],
			["contentLanguage", record.content.contentLanguage],
			["authorizationPresent", record.content.authorizationPresent],
			["cookiePresent", record.content.cookiePresent],
			["cookieCount", record.content.cookieCount],
			["cookieNames", record.content.cookieNames],
			["bodyBytes", record.content.bodyBytes],
			["bodySha256", record.content.bodySha256],
		]);
	}

	if (record.derived !== undefined) {
		pushSection(lines, "derived fingerprints", [
			["requestLine", record.derived.requestLine],
			["headerOrder", record.derived.headerOrder],
			["headerNames", record.derived.headerNames],
			["headerCount", record.derived.headerCount],
			["requestLineSha256", record.derived.requestLineSha256],
			["headerOrderSha256", record.derived.headerOrderSha256],
			["headerNamesSha256", record.derived.headerNamesSha256],
			["headersSha256", record.derived.headersSha256],
			["userAgentSha256", record.derived.userAgentSha256],
			["clientHintsSha256", record.derived.clientHintsSha256],
			["fetchMetadataSha256", record.derived.fetchMetadataSha256],
			["proxySignalsSha256", record.derived.proxySignalsSha256],
		]);
	}

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
