import { format } from 'date-fns';

const UID_MAP: Record<number, string> = {
  0: 'root',
  1000: 'user',
};

export function resolveUid(uid: number | undefined): string {
  if (uid === undefined) return '';
  return UID_MAP[uid] ?? String(uid);
}

export function formatTime(ts: string): string {
  return format(new Date(ts), 'HH:mm:ss');
}

export function formatSize(bytes: number | undefined): string {
  if (bytes === undefined) return '';
  if (bytes < 1024) return `${bytes}B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)}MB`;
}

export function formatMode(mode: number | undefined): string {
  if (mode === undefined) return '';
  return (mode & 0o7777).toString(8).padStart(4, '0');
}

export function truncatePath(path: string, maxLen: number = 50): string {
  if (path.length <= maxLen) return path;
  return '...' + path.slice(-(maxLen - 3));
}

export function hasSetuid(mode: number | undefined): boolean {
  return mode !== undefined && (mode & 0o4000) !== 0;
}

export function hasSetgid(mode: number | undefined): boolean {
  return mode !== undefined && (mode & 0o2000) !== 0;
}

export function isExecutable(mode: number | undefined): boolean {
  return mode !== undefined && (mode & 0o111) !== 0;
}

export type KeyHandler = (e: KeyboardEvent) => void;

export function createKeyboardHandler(handlers: Record<string, () => void>): KeyHandler {
  return (e: KeyboardEvent) => {
    // Ignore if typing in input
    if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
      if (e.key !== 'Escape') return;
    }

    const handler = handlers[e.key];
    if (handler) {
      e.preventDefault();
      handler();
    }
  };
}
