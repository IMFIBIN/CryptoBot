const $ = (id) => document.getElementById(id);

/* ========== i18n ========== */
const dict = {
    en: {
        buy: 'Buy',
        pay: 'Pay',
        spend: 'Spend',
        scenario: 'Scenario',
        calculate: 'Calculate',
        summary: 'Summary',
        allocation: 'Allocation',
        serverOK: 'Server: OK',
        serverFail: 'Server is unreachable',
        pair: 'Pair',
        receive: 'Receive',
        unspent: 'Unspent (not used due to orderbook depth)',
        currentTime: 'Current time',
        scenario2: 'Scenario',
        spendLabel: 'Spend',
        avgPrice: 'Average execution price',
        assetsNoFees: 'Assets cost (no fees)',
        fees: 'Fees',
        totalToPay: 'Total to pay',
        exchange: 'Exchange',
        amountCol: 'Amount',
        priceCol: 'Price',
        feeCol: 'Fee',
        total: 'Total',
        per: 'per 1',
        best_single: 'Best single',
        equal_split: 'Equal split',
        optimal: 'Optimal',
        usdt: 'USDT',
        calculating: 'Calculating…',
    },
    ru: {
        buy: 'Купить',
        pay: 'Оплатить',
        spend: 'Сумма',
        scenario: 'Сценарий',
        calculate: 'Рассчитать',
        summary: 'Результат',
        allocation: 'Распределение',
        serverOK: 'Сервер: OK',
        serverFail: 'Сервер недоступен',
        pair: 'Пара',
        receive: 'Получите',
        unspent: 'Не израсходовано (из-за глубины стакана)',
        currentTime: 'Текущее время',
        scenario2: 'Сценарий',
        spendLabel: 'Затраты',
        avgPrice: 'Средняя цена исполнения',
        assetsNoFees: 'Стоимость активов (без комиссий)',
        fees: 'Комиссии',
        totalToPay: 'Итого к оплате',
        exchange: 'Биржа',
        amountCol: 'Количество',
        priceCol: 'Цена',
        feeCol: 'Комиссия',
        total: 'Итого',
        per: 'за 1',
        best_single: 'Лучшая одиночная',
        equal_split: 'Равное распределение',
        optimal: 'Оптимально',
        usdt: 'USDT',
        calculating: 'Расчёт…',
    }
};

let currentLang = localStorage.getItem('lang') || 'en';

function setLang(lang){
    currentLang = lang;
    localStorage.setItem('lang', lang);

    // toggle button highlight
    document.querySelectorAll('#lang-toggle .lang-btn').forEach(btn=>{
        btn.classList.toggle('active', btn.id === `btn-${lang}`);
    });

    // form labels
    $('lbl-buy').textContent = dict[lang].buy;
    $('lbl-pay').textContent = dict[lang].pay;
    $('lbl-spend').textContent = dict[lang].spend; // без (USDT)
    $('lbl-scenario').textContent = dict[lang].scenario;
    $('calc-btn').textContent = dict[lang].calculate;

    // scenario options text
    const optMap = {
        best_single: dict[lang].best_single,
        equal_split: dict[lang].equal_split,
        optimal: dict[lang].optimal
    };
    const sel = $('scenario');
    Array.from(sel.options).forEach(o => { o.text = optMap[o.value] || o.text; });

    // обновим health надпись, если не bad
    const health = $('health');
    if (health && !health.classList.contains('bad')) {
        health.textContent = dict[lang].serverOK;
    }
}

/* ========== Format helpers ========== */
// Разделитель тысяч — точка, независимо от локали (1.000.000)
const formatThousandsDots = (digits) =>
    digits.replace(/\D/g, '').replace(/\B(?=(\d{3})+(?!\d))/g, '.');

const parseThousandsDots = (val) => {
    // превращаем "1.234.567" -> 1234567 (число)
    const digits = String(val || '').replace(/\./g, '').replace(/\s/g, '');
    const n = parseInt(digits, 10);
    return Number.isFinite(n) ? n : 0;
};

const moneyUSDT = (n) => Number(n).toLocaleString(currentLang === 'ru' ? 'ru-RU' : 'en-US',
    { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const qtyBASE   = (n) => Number(n).toLocaleString(currentLang === 'ru' ? 'ru-RU' : 'en-US',
    { minimumFractionDigits: 6, maximumFractionDigits: 6 });
const priceUSDT = (n) => Number(n).toLocaleString(currentLang === 'ru' ? 'ru-RU' : 'en-US',
    { minimumFractionDigits: 2, maximumFractionDigits: 2 });

const sumAmount = (legs) => (Array.isArray(legs) ? legs : [])
    .reduce((s, l) => s + Number((l && l.amount) || 0), 0);
const sumFees = (legs) => (Array.isArray(legs) ? legs : [])
    .reduce((s, l) => s + Number((l && l.fee) || 0), 0);

/* ========== Backend helpers ========== */
async function checkHealth() {
    const node = $('health');
    try {
        const r = await fetch('/api/health', { cache: 'no-store' });
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        const j = await r.json();
        node.textContent = j?.status ? dict[currentLang].serverOK : dict[currentLang].serverFail;
        node.classList.remove('muted', 'bad');
    } catch {
        node.textContent = dict[currentLang].serverFail;
        node.classList.add('bad');
    }
}

async function loadSymbols() {
    const baseSel = $('base');
    const quoteSel = $('quote');

    try {
        const r = await fetch('/api/symbols', { cache: 'no-store' });
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        const j = await r.json();

        const bases  = Array.isArray(j?.bases)  ? j.bases  : [];
        const quotes = Array.isArray(j?.quotes) ? j.quotes : [];

        baseSel.innerHTML  = bases.map(b => `<option value="${b}">${b}</option>`).join('');
        quoteSel.value = (quotes.includes('USDT') ? 'USDT' : (quotes[0] || 'USDT'));
    } catch {
        baseSel.innerHTML  = ['BTC','ETH','BNB','SOL','XRP','ADA','DOGE','TON','TRX','DOT']
            .map(b => `<option value="${b}">${b}</option>`).join('');
        quoteSel.value = 'USDT';
    }
}

/* ========== Rendering ========== */
function scenarioTitle(code){ return dict[currentLang][code] || code; }

function renderSummary(card, j) {
    const legs = Array.isArray(j.legs) ? j.legs : [];
    const received = sumAmount(legs);
    const assetsNoFees = Number(j.totalCost || 0) - Number(j.totalFees || 0);

    const t = dict[currentLang];

    card.classList.remove('hidden');
    card.innerHTML = `
    <h2>${t.summary}</h2>
    <div class="grid-2">
      <div>
        <div><strong>${t.pair}:</strong> ${j.base || '-'} / ${j.quote || '-'}</div>
        <div><strong>${t.receive}:</strong> ${qtyBASE(received)} ${j.base || ''}</div>
        <div><strong>${t.unspent}:</strong> ${moneyUSDT(j.unspent || 0)} ${t.usdt}</div>
        <div><strong>${t.currentTime}:</strong> ${j.generatedAt || ''}</div>
      </div>
      <div>
        <div><strong>${t.scenario2}:</strong> ${scenarioTitle(j.scenario || '')}</div>
        <div><strong>${t.spendLabel}:</strong> ${moneyUSDT(j.amount || 0)} ${t.usdt}</div>
        <div><strong>${t.avgPrice}:</strong> ${priceUSDT(j.vwap || 0)} ${t.usdt} ${t.per} ${j.base || ''}</div>
        <div><strong>${t.assetsNoFees}:</strong> ${moneyUSDT(assetsNoFees)} ${t.usdt}</div>
        <div><strong>${t.fees}:</strong> ${moneyUSDT(j.totalFees || 0)} ${t.usdt}</div>
        <div><strong>${t.totalToPay}:</strong> ${moneyUSDT(j.totalCost || 0)} ${t.usdt}</div>
      </div>
    </div>
  `;
}

function renderAllocation(card, j) {
    const legs = Array.isArray(j.legs) ? j.legs : [];

    let bestIdx = -1, worstIdx = -1;
    legs.forEach((l, i) => {
        if (typeof l?.price !== 'number' || !isFinite(l.price)) return;
        if (bestIdx === -1 || l.price < legs[bestIdx].price) bestIdx = i;
        if (worstIdx === -1 || l.price > legs[worstIdx].price) worstIdx = i;
    });

    const rows = legs.map((l, i) => {
        const cls = i === bestIdx ? 'best-row' : i === worstIdx ? 'worst-row' : '';
        return `<tr class="${cls}">
      <td>${l.exchange || '-'}</td>
      <td class="num">${qtyBASE(l.amount || 0)}</td>
      <td class="num">${priceUSDT(l.price || 0)}</td>
      <td class="num">${moneyUSDT(l.fee || 0)}</td>
    </tr>`;
    }).join('');

    const t = dict[currentLang];

    card.classList.remove('hidden');
    card.innerHTML = `
    <h2>${t.allocation}</h2>
    <table>
      <thead>
        <tr>
          <th>${t.exchange}</th>
          <th class="num">${t.amountCol} (${j.base || '-'})</th>
          <th class="num">${t.priceCol} (${t.usdt}/${j.base || '-'})</th>
          <th class="num">${t.feeCol} (${t.usdt})</th>
        </tr>
      </thead>
      <tbody>${rows}</tbody>
      <tfoot>
        <tr>
          <th>${t.total}</th>
          <th class="num">${qtyBASE(sumAmount(legs))}</th>
          <th></th>
          <th class="num">${moneyUSDT(sumFees(legs))}</th>
        </tr>
      </tfoot>
    </table>
  `;
}

/* ========== Boot ========== */
document.addEventListener('DOMContentLoaded', () => {
    // init lang toggle
    $('btn-en').addEventListener('click', () => setLang('en'));
    $('btn-ru').addEventListener('click', () => setLang('ru'));
    setLang(currentLang);

    checkHealth();
    loadSymbols();

    // Маска разделения тысяч в поле суммы (1.000.000)
    const amt = $('amount');
    if (amt) {
        // первичная нормализация
        amt.value = formatThousandsDots(String(amt.value || ''));

        amt.addEventListener('input', () => {
            const selEnd = amt.selectionEnd;
            amt.value = formatThousandsDots(amt.value);
            // курсор в конец — просто и надёжно
            amt.setSelectionRange(amt.value.length, amt.value.length);
        });
        amt.addEventListener('blur', () => {
            amt.value = formatThousandsDots(amt.value);
        });
    }

    const form = $('plan-form');
    const res  = $('result');
    const legs = $('legs');
    const btn  = $('calc-btn');

    form?.addEventListener('submit', async (e) => {
        e.preventDefault();
        if (!btn) return;
        btn.disabled = true;

        const base = $('base')?.value?.trim().toUpperCase() || 'BTC';
        const quote = $('quote')?.value?.trim().toUpperCase() || 'USDT';
        const scenario = $('scenario')?.value || 'best_single';
        const amount = parseThousandsDots($('amount')?.value || '0'); // из 1.000.000 -> 1000000

        res.classList.remove('hidden');
        res.innerHTML = `<div class="muted">${dict[currentLang].calculating}</div>`;
        legs.classList.add('hidden');
        legs.innerHTML = '';

        try {
            // depth больше не отправляем — сервер получит 0 (безлимит)
            const r = await fetch('/api/plan', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ base, quote, amount, scenario })
            });

            const text = await r.text();
            if (!r.ok) throw new Error(text || `HTTP ${r.status}`);

            let j;
            try { j = JSON.parse(text); }
            catch { throw new Error(`Bad JSON: ${text.slice(0, 400)}`); }

            if (j && j.error) {
                res.innerHTML = `<h2>Error</h2><pre>${j.error}</pre>`;
                return;
            }

            renderSummary(res, j || {});
            renderAllocation(legs, j || {});
        } catch (err) {
            res.innerHTML = `<h2>Error</h2><pre>${String(err)}</pre>`;
        } finally {
            btn.disabled = false;
        }
    });
});
