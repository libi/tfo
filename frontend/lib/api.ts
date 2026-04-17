const API_BASE = '/api';

export interface Fragment {
    id: string;
    title: string;
    content: string;
    tags: string[];
    createdAt: string;
    filePath?: string;
}

export interface NoteSummary {
    id: string;
    title: string;
    tags: string[];
    createdAt: string;
    preview: string;
}

export interface TagCount {
    tag: string;
    count: number;
}

export interface HeatmapEntry {
    date: string;
    count: number;
}

export interface HighlightRange {
    start: number;
    end: number;
}

export interface HighlightedFragment {
    text: string;
    highlights?: HighlightRange[];
}

export interface SearchResult {
    id: string;
    title: string;
    score: number;
    fragments: HighlightedFragment[];
}

export interface WeChatConfig {
    enabled: boolean;
    baseUrl: string;
    token: string;
    cdnBaseUrl?: string;
    autoConnect: boolean;
    pollTimeoutSeconds: number;
    reconnectIntervalSec: number;
}

export interface AppConfig {
    dataDir: string;
    uiLanguage?: string;
    hotkeyGlobalQuickCapture: string;
    hotkeyInputFocus: string;
    hotkeySave: string;
    wechat: WeChatConfig;
    indexRebuildOnStart: boolean;
    titleMinContentLength: number;
}

// --- Notes ---

export async function createNote(content: string): Promise<Fragment> {
    const res = await fetch(`${API_BASE}/notes`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content }),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

export async function getNote(id: string): Promise<Fragment> {
    const res = await fetch(`${API_BASE}/notes/${id}`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

export async function updateNote(id: string, content: string): Promise<Fragment> {
    const res = await fetch(`${API_BASE}/notes/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content }),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

export async function deleteNote(id: string): Promise<void> {
    const res = await fetch(`${API_BASE}/notes/${id}`, { method: 'DELETE' });
    if (!res.ok) throw new Error(await res.text());
}

export async function listNotesByDate(date: string): Promise<NoteSummary[]> {
    const res = await fetch(`${API_BASE}/notes?date=${date}`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

export async function listNotesByMonth(month: string): Promise<NoteSummary[]> {
    const res = await fetch(`${API_BASE}/notes?month=${month}`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

export async function listNotesRecent(offset = 0, limit = 20): Promise<{ items: NoteSummary[]; total: number }> {
    const res = await fetch(`${API_BASE}/notes?offset=${offset}&limit=${limit}`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

// --- Tags ---

export async function getAllTags(): Promise<TagCount[]> {
    const res = await fetch(`${API_BASE}/tags`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

// --- Heatmap ---

export async function getHeatmap(month: string): Promise<HeatmapEntry[]> {
    const res = await fetch(`${API_BASE}/heatmap?month=${month}`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

// --- Search ---

export async function searchNotes(query: string, limit = 20, offset = 0): Promise<{ results: SearchResult[]; total: number }> {
    const res = await fetch(`${API_BASE}/search?q=${encodeURIComponent(query)}&limit=${limit}&offset=${offset}`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

export async function rebuildIndex(): Promise<{ message: string }> {
    const res = await fetch(`${API_BASE}/search/rebuild`, { method: 'POST' });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

// --- Config ---

export async function getConfig(): Promise<AppConfig> {
    const res = await fetch(`${API_BASE}/config`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

export async function updateConfig(config: AppConfig): Promise<AppConfig> {
    const res = await fetch(`${API_BASE}/config`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

export async function updateBootstrap(dataDir: string): Promise<{ dataDir: string; message: string }> {
    const res = await fetch(`${API_BASE}/bootstrap`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ dataDir }),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

// --- Directory Browse ---

export interface DirEntry {
    name: string;
    path: string;
}

export interface BrowseDirectoryResult {
    current: string;
    parent: string;
    dirs: DirEntry[];
}

export async function browseDirectory(path?: string): Promise<BrowseDirectoryResult> {
    const res = await fetch(`${API_BASE}/browse-directory`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path: path || '' }),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

// --- WeChat ---

export async function getChannelStates(): Promise<Record<string, string>> {
    const res = await fetch(`${API_BASE}/wechat/states`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

export async function startWeChat(): Promise<void> {
    const res = await fetch(`${API_BASE}/wechat/start`, { method: 'POST' });
    if (!res.ok) throw new Error(await res.text());
}

export async function stopWeChat(): Promise<void> {
    const res = await fetch(`${API_BASE}/wechat/stop`, { method: 'POST' });
    if (!res.ok) throw new Error(await res.text());
}

export async function getWeChatQRCode(baseUrl?: string): Promise<{ qrcode: string; qrcodeImgContent: string }> {
    const params = baseUrl ? `?baseUrl=${encodeURIComponent(baseUrl)}` : '';
    const res = await fetch(`${API_BASE}/wechat/qrcode${params}`);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

export async function pollWeChatQRCode(qrcode: string, baseUrl?: string): Promise<{
    status: string;
    botToken?: string;
    botId?: string;
    baseUrl?: string;
}> {
    const res = await fetch(`${API_BASE}/wechat/qrcode/poll`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ qrcode, baseUrl }),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

export async function loginWithQRCode(botToken: string, botId: string, baseUrl: string): Promise<{ status: string }> {
    const res = await fetch(`${API_BASE}/wechat/qrcode/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ botToken, botId, baseUrl }),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}
