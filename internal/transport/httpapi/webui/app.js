const $ = (id) => document.getElementById(id);

/* ========== i18n ========== */
const dict = {
    en: {
        buy: 'Buy',
        pay: 'Pay',
        spend: 'Spend',
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
        total: 'Total',
        per: 'per 1',
        best_single: 'Best single',
        equal_split: 'Equal split',
        optimal: 'Optimal',
        usdt: 'USDT',
        calculating: 'Calculating…',
        resultsFor: 'Results for',
    },
    ru: {
        buy: 'Купить',
        pay: 'Оплатить',
        spend: 'Сумма',
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
        total: 'Итого',
        per: 'за 1',
        best_single: 'Лучшая одиночная',
        equal_split: 'Равное распределение',
        optimal: 'Оптимально',
        usdt: 'USDT',
        calculating: 'Расчёт…',
        resultsFor: 'Результаты для',
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
    $('lbl-spend').textContent = dict[lang].spend;
    $('calc-btn').textContent = dict[lang].calculate;

    // обновим health надпись, если не bad
    const health = $('health');
    if (health && !health.classList.contains('bad')) {
        health.textContent = dict[lang].serverOK;
    }
}

/* ========== Format helpers ========== */
// Разделитель тысяч — точка (1.000.000)
const formatThousandsDots = (digits) =>
    digits.replace(/\D/g, '').replace(/\B(?=(\d{3})+(?!\d))/g, '.');

const parseThousandsDots = (val) => {
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

/* ========== Rendering (HTML builders) ========== */
function scenarioTitle(code){ return dict[currentLang][code] || code; }

function buildSummaryHTML(j){
    const legs = Array.isArray(j.legs) ? j.legs : [];
    const received = sumAmount(legs);
    const assetsNoFees = Number(j.totalCost || 0) - Number(j.totalFees || 0);

    const t = dict[currentLang];
    const unspentBlock = (Number(j.unspent || 0) > 0.0000001)
        ? `<div><strong>${t.unspent}:</strong> ${moneyUSDT(j.unspent || 0)} ${t.usdt}</div>`
        : '';

    // описания для сценариев
    const descriptions = {
        best_single: currentLang === 'ru'
            ? 'Вся сумма уходит на одну биржу с наилучшей ценой'
            : 'All funds go to the single exchange with the best price',
        equal_split: currentLang === 'ru'
            ? 'Сумма делится равными частями между всеми биржами'
            : 'Funds are split equally across all exchanges',
        optimal: currentLang === 'ru'
            ? 'Сумма распределяется оптимально между биржами для минимальной цены исполнения'
            : 'Funds are distributed optimally across exchanges for the best execution price'
    };

    const descr = descriptions[j.scenario] || '';

    return `
    <h2>${t.summary} — ${scenarioTitle(j.scenario || '')}</h2>
    <p class="muted" style="margin-top:-6px;margin-bottom:12px;">${descr}</p>
    <div class="grid-2">
      <div>
        <div><strong>${t.pair}:</strong> ${j.base || '-'} / ${j.quote || '-'}</div>
        <div><strong>${t.receive}:</strong> ${qtyBASE(received)} ${j.base || ''}</div>
        ${unspentBlock}
        <div><strong>${t.currentTime}:</strong> ${j.generatedAt || ''}</div>
      </div>
      <div>
        <div><strong>${t.spendLabel}:</strong> ${moneyUSDT(j.amount || 0)} ${t.usdt}</div>
        <div><strong>${t.avgPrice}:</strong> ${priceUSDT(j.vwap || 0)} ${t.usdt} ${t.per} ${j.base || ''}</div>
        <div><strong>${t.assetsNoFees}:</strong> ${moneyUSDT(assetsNoFees)} ${t.usdt}</div>
        <div><strong>${t.totalToPay}:</strong> ${moneyUSDT(j.totalCost || 0)} ${t.usdt}</div>
      </div>
    </div>
  `;
}


function buildAllocationHTML(j){
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
    </tr>`;
    }).join('');

    const t = dict[currentLang];

    return `
    <h2>${t.allocation}</h2>
    <table>
      <thead>
        <tr>
          <th>${t.exchange}</th>
          <th class="num">${t.amountCol} (${j.base || '-'})</th>
          <th class="num">${t.priceCol} (${t.usdt}/${j.base || '-'})</th>
        </tr>
      </thead>
      <tbody>${rows}</tbody>
      <tfoot>
        <tr>
          <th>${t.total}</th>
          <th class="num">${qtyBASE(sumAmount(legs))}</th>
          <th></th>
        </tr>
      </tfoot>
    </table>
  `;
}

function buildScenarioPanel(j){
    return `
    <section class="card">
      ${buildSummaryHTML(j)}
      ${buildAllocationHTML(j)}
    </section>
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

    // Маска разделения тысяч (1.000.000)
    const amt = $('amount');
    if (amt) {
        amt.value = formatThousandsDots(String(amt.value || ''));
        amt.addEventListener('input', () => {
            amt.value = formatThousandsDots(amt.value);
            amt.setSelectionRange(amt.value.length, amt.value.length);
        });
        amt.addEventListener('blur', () => {
            amt.value = formatThousandsDots(amt.value);
        });
    }

    const form = $('plan-form');
    const cmp  = $('comparisons'); // контейнер для 3 сценариев
    const btn  = $('calc-btn');

    form?.addEventListener('submit', async (e) => {
        e.preventDefault();
        if (!btn) return;
        btn.disabled = true;

        const base = $('base')?.value?.trim().toUpperCase() || 'BTC';
        const quote = $('quote')?.value?.trim().toUpperCase() || 'USDT';
        const amount = parseThousandsDots($('amount')?.value || '0');

        // Прелоадер
        cmp.innerHTML = `<section class="card"><div class="muted">${dict[currentLang].calculating}</div></section>`;

        const scenarios = ['best_single', 'equal_split', 'optimal'];

        try {
            // параллельно считаем все сценарии
            const reqBody = (scenario) => JSON.stringify({ base, quote, amount, scenario });
            const fetchOne = (scenario) =>
                fetch('/api/plan', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: reqBody(scenario)
                }).then(async r => {
                    const text = await r.text();
                    if (!r.ok) throw new Error(text || `HTTP ${r.status}`);
                    const j = JSON.parse(text);
                    if (j && j.error) throw new Error(j.error);
                    return j;
                });

            const results = await Promise.all(scenarios.map(fetchOne));

            // отрисовка: заголовок + три панели
            const title = `<h2 class="muted" style="margin:8px 0 0 2px;">${dict[currentLang].resultsFor}: ${base}/${quote}</h2>`;
            cmp.innerHTML = title + results.map(buildScenarioPanel).join('');

        } catch (err) {
            cmp.innerHTML = `<section class="card"><h2>Error</h2><pre>${String(err)}</pre></section>`;
        } finally {
            btn.disabled = false;
        }
    });
});
