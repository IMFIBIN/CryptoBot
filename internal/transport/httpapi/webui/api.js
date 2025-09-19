import { dict, currentLang } from './i18n.js';

const $ = (id) => document.getElementById(id);

function updateFooterStatus(ok) {
    const el = $('footer-status');
    if (!el) return;
    el.textContent = ok ? dict[currentLang].serverOK : dict[currentLang].serverFail;
}

export async function checkHealth() {
    const node = $('health');
    if (!node) return;
    try {
        const r = await fetch('/api/health', { cache: 'no-store' });
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        const j = await r.json();
        node.textContent = j?.status ? dict[currentLang].serverOK : dict[currentLang].serverFail;
        node.classList.toggle('bad', !j?.status);
        updateFooterStatus(Boolean(j?.status));
    } catch {
        node.textContent = dict[currentLang].serverFail;
        node.classList.add('bad');
        updateFooterStatus(false);
    }
}

export async function loadSymbols() {
    const baseSel = $('base');
    const quoteSel = $('quote');
    if (!baseSel || !quoteSel) return;
    try {
        const r = await fetch('/api/symbols', { cache: 'no-store' });
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        const j = await r.json();
        const bases = Array.isArray(j?.bases) ? j.bases : [];
        let quotes = Array.isArray(j?.quotes) ? j.quotes : [];
        if (!quotes.length) quotes = bases.slice();
        baseSel.innerHTML = bases.map(b => `<option value="${b}">${b}</option>`).join('');
        quoteSel.innerHTML = quotes.map(q => `<option value="${q}">${q}</option>`).join('');
        if (quotes.includes('USDT')) quoteSel.value = 'USDT'; else quoteSel.selectedIndex = 0;
    } catch {
        const fallback = ['USDT', 'BTC', 'ETH', 'BNB', 'SOL', 'XRP', 'ADA', 'DOGE', 'TON', 'TRX', 'DOT'];
        baseSel.innerHTML = fallback.map(b => `<option value="${b}">${b}</option>`).join('');
        quoteSel.innerHTML = fallback.map(q => `<option value="${q}">${q}</option>`).join('');
        quoteSel.value = 'USDT';
    }
}

export async function plan(base, quote, amount, scenario) {
    const r = await fetch('/api/plan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ base, quote, amount, scenario }),
    });
    const text = await r.text();
    if (!r.ok) throw new Error(text || `HTTP ${r.status}`);
    const j = JSON.parse(text);
    if (j && j.error) throw new Error(j.error);
    return j;
}
