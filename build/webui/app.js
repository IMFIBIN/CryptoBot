// internal/transport/httpapi/webui/app.js
// ES-модуль: UI-слой (рендер и события). Логику локали/форматирования/запросов берём из модулей.
import { dict, currentLang, setLang, scenarioTitle } from './i18n.js';
import {
    moneyUSDT, qtyBASE, priceUSDT, qtyBASE5, qtyQUOTE5, priceUSDT5,
    qtyCOINTerse, formatThousandsDots, parseThousandsDots
} from './format.js';
import { checkHealth, loadSymbols, plan } from './api.js';

const $ = (id) => document.getElementById(id);

/* ────────────────────────────────────────────────────────────────────────────
   Toggle: «Показать выгоду наглядно» (только для сценария 1 — best_single)
   ──────────────────────────────────────────────────────────────────────────── */
const uniformMinQty = new Set();
function toggleUniformMinQty(idx) {
    if (uniformMinQty.has(idx)) uniformMinQty.delete(idx); else uniformMinQty.add(idx);
    rerenderScenario(idx);
}

window.toggleUniformMinQty = toggleUniformMinQty;

/* Синхронизация стиля кнопок: делаем кнопку визуального режима как «Рассчитать» */
function applyCalcStyleToButtons() {
    const calc = $('calc-btn');
    if (!calc) return;
    const cs = getComputedStyle(calc);
    document.querySelectorAll('[data-like-calc-btn]').forEach(btn => {
        if (calc.className) btn.className = calc.className; // тот же класс
        // fallback – копируем ключевые computed-стили
        btn.style.backgroundColor = cs.backgroundColor;
        btn.style.borderColor = cs.borderColor;
        btn.style.color = cs.color;
        btn.style.boxShadow = cs.boxShadow;
    });
}

/* Вспомогательное: суммирование количества BASE по ногам */
function sumBaseAmount(legs, base) {
    const baseU = String(base || '').toUpperCase();
    return (Array.isArray(legs) ? legs : []).reduce((s, l) => s + Number(
        baseU === 'USDT'
            ? (l.usdt ?? l.amountUSDT ?? (Number(l.amount || 0) * Number(l.price || 0)))
            : (l.amount || 0)
    ), 0);
}

/* ────────────────────────────────────────────────────────────────────────────
   Рендер
   ──────────────────────────────────────────────────────────────────────────── */
function buildSummaryHTML(j) {
    const t = dict[currentLang];

    // Всегда показываем фактическую пару BASE/QUOTE
    const base = j.base || '';
    const quote = j.quote || '';

    const received = Number(j.generated || 0);

    // «Обменяли»: сумма и ЕДИНИЦА — это именно QUOTE
    const spentVal   = Number(j.totalCost ?? j.amount ?? 0);
    const spendNum   = qtyCOINTerse(spentVal);
    const spendUnits = quote;

    const unspentVal  = Number(j.unspent || 0);
    const unspentStr  = `${qtyCOINTerse(unspentVal)} ${quote}`;
    const unspentBlock = (unspentVal > 0.0000001)
        ? `<div><strong>${t.unspent}:</strong> ${unspentStr}</div>` : '';

    const descriptions = {
        best_single: currentLang === 'ru'
            ? 'Вкладываем всю сумму в каждую биржу и находим выгодный обмен'
            : 'We invest the entire amount in each exchange and find a profitable exchange.',
        equal_split: currentLang === 'ru'
            ? 'Сумма делится равными частями между всеми биржами'
            : 'Funds are split equally across all exchanges',
        optimal: currentLang === 'ru'
            ? 'Алгоритм распределяет всю сумму между биржами так, что бы обменять с наибольшей выгодой'
            : 'The algorithm distributes the entire amount between exchanges so that it can be exchanged with the greatest benefit.',
    };
    const descr = descriptions[j.scenario] || '';

    return `
    <h2>${t.summary} — ${scenarioTitle(j.scenario || '')}</h2>
    <p class="muted" style="margin-top:-6px;margin-bottom:12px;">${descr}</p>
    <div class="divider"></div>
    <div class="grid-2">
      <div>
        <div><strong>${t.pair}:</strong> ${base || '-'} / ${quote || '-'}</div>
        <div><strong>${t.currentTime}:</strong> ${j.generatedAt || ''}</div>
      </div>
      <div>
        <div><strong>${t.receive}:</strong> ${qtyBASE(received)} ${base}</div>
        <div><strong>${t.spendLabel}:</strong> ${spendNum} ${spendUnits}</div>
        ${unspentBlock}
      </div>
    </div>
  `;
}

function buildAllocationHTML(j, idx) {
    const t = dict[currentLang];
    const base  = j.base || '-';
    const quote = j.quote || '-';
    const baseU  = String(base).toUpperCase();
    const isOptimal = (j.scenario === 'optimal');

    // Сколько BASE на ноге
    const legBaseAmount = (l) => {
        if (baseU === 'USDT') {
            const u = (l && (l.usdt ?? l.amountUSDT));
            if (typeof u !== 'undefined') return Number(u || 0);
            return Number(l.amount || 0) * Number(l.price || 0);
        }
        return Number(l.amount || 0);
    };

    const legsRaw = Array.isArray(j.legs) ? j.legs : [];

    // Сортировка:
    const legs = isOptimal
        ? legsRaw.slice()
        : legsRaw
            .filter(l => Number.isFinite(Number(l?.price)))
            .sort((a, b) => Number(a.price) - Number(b.price));

    // База для «Разницы»
    const bestQty   = (legs.length ? legBaseAmount(legs[0]) : 0);
    const bestPrice = (legs.length ? Number(legs[0].price || 0) : 0);

    // Минимальное реальное количество среди всех строк (для режима «выравнивания»)
    const minQty = legs.length ? Math.min(...legs.map(legBaseAmount)) : 0;

    // Флаг режима «Показать выгоду наглядно» (только для best_single)
    const isBestSingle = (j.scenario === 'best_single');
    const showUniform = isBestSingle && uniformMinQty.has(idx);

    // Заголовки
    const exchangeHeader = t.exchange;
    const amountHeader   = `${t.amountCol} (${quote})`;
    const priceHeader    = `${t.priceCol} (${base})`;
    const diffHeader     = showUniform ? `${t.diffCol} (${quote})` : `${t.diffCol}`;

    // Подсказка о маршруте через USDT — инфо-текст
    const bothNotUsdt = (String(base).toUpperCase() !== 'USDT' && String(quote).toUpperCase() !== 'USDT');
    const viaUsdtNote = bothNotUsdt
        ? `<div class="muted" style="margin:6px 0 10px 0;">
        ${currentLang === 'ru'
            ? `Маршрут через USDT: продаём ${quote} → USDT, затем покупаем ${base} за USDT`
            : `Routed via USDT: sell ${quote} → USDT, then buy ${base} for USDT`}
       </div>`
        : '';

    // Строки
    const rows = legs.map((l, i) => {
        const qtyReal  = legBaseAmount(l);
        const qtyShown = (isOptimal ? qtyReal : (showUniform ? minQty : qtyReal));
        const price    = Number(l.price || 0);

        // Разница: обычный режим — BASE; режим наглядности — QUOTE
        const diffValue = showUniform
            ? (minQty * price - minQty * bestPrice)
            : (qtyReal - bestQty);

        const diffCell = showUniform ? priceUSDT5(diffValue) : qtyBASE5(diffValue);

        if (isOptimal) {
            return `<tr>
        <td>${l.exchange || '-'}</td>
        <td class="num">${qtyBASE5(qtyShown)}</td>
        <td class="num">${priceUSDT5(price)}</td>
      </tr>`;
        }

        const cls = i === 0 ? 'best-row' : (i === legs.length - 1 ? 'worst-row' : '');
        return `<tr class="${cls}">
      <td>${l.exchange || '-'}</td>
      <td class="num">${qtyBASE5(qtyShown)}</td>
      <td class="num">${diffCell}</td>
      <td class="num">${priceUSDT5(price)}</td>
    </tr>`;
    }).join('');

    // Таблица
    const totalBase  = sumBaseAmount(legs.length ? legs : legsRaw, baseU);
    const totalQuote = Number(j.totalCost ?? j.amount ?? 0);

    if (isOptimal) {
        return `
      <div class="divider-small"></div>
      <h2>${t.allocation}</h2>
      ${viaUsdtNote}
      <table class="grid-table">
        <thead>
          <tr>
            <th>${exchangeHeader}</th>
            <th class="num">${amountHeader}</th>
            <th class="num">${priceHeader}</th>
          </tr>
        </thead>
        <tbody>${rows}</tbody>
        <tfoot>
          <tr>
            <th>${t.total}</th>
            <th class="num">${qtyQUOTE5(totalQuote)}</th>
            <th class="num">${qtyBASE5(totalBase)}</th>
          </tr>
        </tfoot>
      </table>
    `;
    }

    return `
    <div class="divider-small"></div>

    <div class="alloc-header" style="display:flex;align-items:center;justify-content:space-between;gap:12px;margin:0 0 8px 0;">
      <h2 style="margin:0;">${t.allocation}</h2>
      ${isBestSingle ? `
        <button
          data-like-calc-btn
          type="button"
          class="${($('calc-btn')?.className || 'btn')}"
          style="padding:6px 10px;border-radius:10px;font-weight:600"
          onclick="toggleUniformMinQty(${idx})">
          ${currentLang === 'ru'
        ? (showUniform ? 'Показать реальные количества' : 'Показать выгоду наглядно')
        : (showUniform ? 'Show actual volumes' : 'Show visual benefit')}
        </button>
      ` : ``}
    </div>

    ${viaUsdtNote}

    <table class="grid-table">
      <thead>
        <tr>
          <th>${exchangeHeader}</th>
          <th class="num">${amountHeader}</th>
          <th class="num">${diffHeader}</th>
          <th class="num">${priceHeader}</th>
        </tr>
      </thead>
      <tbody>${rows}</tbody>
      ${j.scenario === 'best_single' ? '' : `
        <tfoot>
          <tr>
            <th>${t.total}</th>
            <th class="num">${qtyBASE5(totalBase)}</th>
            <th></th>
            <th></th>
          </tr>
        </tfoot>`}
    </table>

    <div class="divider-legend"></div>
    <div style="display:flex; gap:12px; font-size:0.9em; align-items:center;">
      <div style="display:flex; align-items:center; gap:6px;">
        <span style="display:inline-block;width:12px;height:12px;background:#36B37E26;border-radius:2px;"></span>
        <span>${currentLang === 'ru' ? 'Лучший обмен' : 'Best rate'}</span>
      </div>
      <div style="display:flex; align-items:center; gap:6px;">
        <span style="display:inline-block;width:12px;height:12px;background:#EB575726;border-radius:2px;"></span>
        <span>${currentLang === 'ru' ? 'Худший обмен' : 'Worst rate'}</span>
      </div>
    </div>
  `;
}

function buildScenarioPanel(j, idx) {
    return `
    <section class="card">
      ${buildSummaryHTML(j)}
      ${buildAllocationHTML(j, idx)}
    </section>
  `;
}

/* ────────────────────────────────────────────────────────────────────────────
   Инициализация UI
   ──────────────────────────────────────────────────────────────────────────── */
document.addEventListener('DOMContentLoaded', () => {
    const y = $('footer-year');
    if (y) y.textContent = new Date().getFullYear();

    setLang(currentLang);
    $('btn-en')?.addEventListener('click', () => setLang('en'));
    $('btn-ru')?.addEventListener('click', () => setLang('ru'));

    const amountInput = $('amount');
    if (amountInput) {
        amountInput.addEventListener('input', (e) => {
            const caret = e.target.selectionStart;
            const digits = e.target.value.replace(/\./g, '');
            e.target.value = formatThousandsDots(digits);
            e.target.selectionStart = e.target.selectionEnd = Math.min(caret + 1, e.target.value.length);
        });
    }

    checkHealth();
    loadSymbols();

    const form = $('plan-form');
    const cmp  = $('comparisons');
    const btn  = $('calc-btn');

    form?.addEventListener('submit', async (e) => {
        e.preventDefault();
        if (!btn || !cmp) return;
        btn.disabled = true;

        const base   = $('base')?.value?.trim().toUpperCase() || 'BTC';
        const quote  = $('quote')?.value?.trim().toUpperCase() || 'USDT';
        const amount = parseThousandsDots($('amount')?.value || '0');

        cmp.innerHTML = `<section class="card"><div class="muted">${dict[currentLang].calculating}</div></section>`;

        const scenarios = ['best_single', 'equal_split', 'optimal'];
        try {
            const results = await Promise.all(scenarios.map(s => plan(base, quote, amount, s)));
            window._lastResults = results;

            const title = `<h2 class="muted" style="margin:8px 0 0 2px;">${dict[currentLang].resultsFor}: ${base}/${quote}</h2>`;
            const panels = results.map((r, i) => buildScenarioPanel(r, i)).join('');
            cmp.innerHTML = title + panels;

            applyCalcStyleToButtons();
        } catch (err) {
            let msg = String(err);
            try {
                const parsed = JSON.parse(msg);
                if (parsed && parsed.error) msg = parsed.error;
            } catch {}
            cmp.innerHTML = `<section class="card"><div style="color:#c008e8;font-weight:600">
        Ошибка: ${msg}
      </div></section>`;
        } finally {
            btn.disabled = false;
        }
    });

    // локальный ререндер конкретного сценария без повторных запросов
    window.rerenderScenario = function(i) {
        if (!Array.isArray(window._lastResults)) return;
        const cmp = $('comparisons');
        if (!cmp) return;

        const base = $('base')?.value?.trim().toUpperCase() || 'BTC';
        const quote = $('quote')?.value?.trim().toUpperCase() || 'USDT';

        const title = `<h2 class="muted" style="margin:8px 0 0 2px;">${dict[currentLang].resultsFor}: ${base}/${quote}</h2>`;
        const panels = window._lastResults.map((r, idx) => buildScenarioPanel(r, idx)).join('');
        cmp.innerHTML = title + panels;

        applyCalcStyleToButtons();
    };
});
