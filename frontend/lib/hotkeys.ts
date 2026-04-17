const MODIFIER_ORDER = ['Ctrl', 'Alt', 'Shift', 'Cmd'] as const;

function normalizeMainKey(key: string) {
    if (!key) return '';
    if (key === ' ') return 'Space';
    if (key === 'Escape') return 'Esc';
    if (key.length === 1) return key.toUpperCase();
    return key[0].toUpperCase() + key.slice(1);
}

export function normalizeShortcut(shortcut: string) {
    const parts = shortcut
        .split('+')
        .map(part => part.trim())
        .filter(Boolean)
        .map(part => {
            const lower = part.toLowerCase();
            if (lower === 'control' || lower === 'ctrl') return 'Ctrl';
            if (lower === 'option' || lower === 'alt') return 'Alt';
            if (lower === 'shift') return 'Shift';
            if (lower === 'meta' || lower === 'cmd' || lower === 'command') return 'Cmd';
            return normalizeMainKey(part);
        });

    const modifiers = MODIFIER_ORDER.filter(modifier => parts.includes(modifier));
    const mainKeys = parts.filter(part => !MODIFIER_ORDER.includes(part as (typeof MODIFIER_ORDER)[number]));

    return [...modifiers, ...mainKeys].join('+');
}

export function keyboardEventToShortcut(event: KeyboardEvent | React.KeyboardEvent<HTMLElement>) {
    const parts: string[] = [];
    if (event.ctrlKey) parts.push('Ctrl');
    if (event.altKey) parts.push('Alt');
    if (event.shiftKey) parts.push('Shift');
    if (event.metaKey) parts.push('Cmd');

    const rawKey = event.key;
    const modifierOnlyKeys = new Set(['Control', 'Alt', 'Shift', 'Meta', 'Command']);
    if (!modifierOnlyKeys.has(rawKey)) {
        // On macOS, Alt (Option) modifies event.key to special characters (e.g. Alt+S → 'ß').
        // Use event.code to get the physical key for reliable matching.
        const hasModifier = event.ctrlKey || event.altKey || event.shiftKey || event.metaKey;
        const code = 'code' in event ? (event as KeyboardEvent).code : '';
        let mainKey: string;
        if (hasModifier && code) {
            if (code.startsWith('Key')) {
                mainKey = code.slice(3).toUpperCase();
            } else if (code.startsWith('Digit')) {
                mainKey = code.slice(5);
            } else {
                // For special keys like Enter, Space, etc., use normalized key name
                mainKey = normalizeMainKey(code);
            }
        } else {
            mainKey = normalizeMainKey(rawKey);
        }
        if (mainKey) {
            parts.push(mainKey);
        }
    }

    return normalizeShortcut(parts.join('+'));
}

export function matchesShortcut(event: KeyboardEvent, shortcut: string) {
    const normalizedTarget = normalizeShortcut(shortcut);
    if (!normalizedTarget) return false;
    return keyboardEventToShortcut(event) === normalizedTarget;
}