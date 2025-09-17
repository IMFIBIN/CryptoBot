const $ = (id) => document.getElementById(id);

/* ========== i18n (минимально необходимый набор фраз) ========== */
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
        unspent: 'Unspent (orderbook depth limit)',
        currentTime: 'Current time',
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
        pay: 'Оплачивать',
        spend: 'Потратить',
        calculate: 'Рассчитать',
        summary: 'Результат',
        allocation: 'Распределение',
        serverOK: 'Сервер работает',
        serverFail: 'Сервер не отвечает',
        pair: 'Пара',
        receive: 'Получите',
        unspent: 'Не израсходовано (ограничение глубины)',
        currentTime: 'Текущее время',
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

let currentLang = localStorage.getItem('lang') || 'ru';

function setLang(lang){
    currentLang = lang;
    localStorage.setItem('lang', lang);

    // Подсветка кнопок языка (если есть)
    document.querySelectorAll('#lang-toggle .lang-btn')?.forEach(btn=>{
        btn.classList.toggle('active', btn.id === `btn-${lang}`);
    });

    // Лейблы формы (если есть такие элементы)
    $('lbl-buy')  && ($('lbl-buy').textContent  = dict[lang].buy);
    $('lbl-pay')  && ($('lbl-pay').textContent  = dict[lang].pay);
    $('lbl-spend')&& ($('lbl-spend').textContent= dict[lang].spend);
    $('calc-btn') && ($('calc-btn').textContent = dict[lang].calculate);

    // Health-надпись, если сервер ОК
    const health = $('health');
    if (health && !health.classList.contains('bad')) {
        health.textContent = dict[lang].serverOK;
    }
    // Футер
    const fs = $('footer-status');
    if (fs && !health?.classList.contains('bad')) {
        fs.textContent = dict[lang].serverOK;
    }
}

/* ========== Форматирование ========== */
// 1 000 000 → "1.000.000"
const formatThousandsDots = (digits) =>
    digits.replace(/\D/g, '').replace(/\B(?=(\d{3})+(?!\d))/g, '.');

const parseThousandsDots = (val) => {
    const digits = String(val || '').replace(/\./g, '').replace(/\s/g, '');
    const n = parseInt(digits, 10);
    return Number.isFinite(n) ? n : 0;
};

const moneyUSDT = (n) => Number(n).toLocaleString(
    currentLang === 'ru' ? 'ru-RU' : 'en-US',
    { minimumFractionDigits: 2, maximumFractionDigits: 2 }
);
const qtyBASE   = (n) => Number(n).toLocaleString(
    currentLang === 'ru' ? 'ru-RU' : 'en-US',
    { minimumFractionDigits: 6, maximumFractionDigits: 6 }
);
const priceUSDT = (n) => Number(n).toLocaleString(
    currentLang === 'ru' ? 'ru-RU' : 'en-US',
    { minimumFractionDigits: 2, maximumFractionDigits: 2 }
);

// Убираем хвостовые нули у уже форматированной строки ("1,000000" -> "1")
function trimZerosTail(str){
    return str.replace(/([,.]\d*?)0+$/,'$1').replace(/[,.]$/,'');
}

// Формат короткого отображения количества монеты (для сумм в валюте оплаты в кросс-парах)
function qtyCOINTerse(n){
    return trimZerosTail(qtyBASE(n));
}

const sumAmount = (legs) => (Array.isArray(legs) ? legs : [])
    .reduce((s, l) => s + Number((l && l.amount) || 0), 0);

/* ========== Footer helpers ========== */
function updateFooterStatus(ok) {
    const el = $('footer-status');
    if (!el) return;
    el.textContent = ok ? dict[currentLang].serverOK : dict[currentLang].serverFail;
}

/* ========== Health ========== */
async function checkHealth() {
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

/* ========== Symbols ========== */
async function loadSymbols() {
    const baseSel  = $('base');
    const quoteSel = $('quote');
    if (!baseSel || !quoteSel) return;

    try {
        const r = await fetch('/api/symbols', { cache: 'no-store' });
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        const j = await r.json();

        const bases  = Array.isArray(j?.bases)  ? j.bases  : [];
        let   quotes = Array.isArray(j?.quotes) ? j.quotes : [];

        if (!quotes.length) quotes = bases.slice();

        baseSel.innerHTML  = bases.map(b  => `<option value="${b}">${b}</option>`).join('');
        quoteSel.innerHTML = quotes.map(q => `<option value="${q}">${q}</option>`).join('');

        // дефолт для "Оплатить": USDT если есть
        if (quotes.includes('USDT')) quoteSel.value = 'USDT';
        else quoteSel.selectedIndex = 0;
    } catch {
        const fallback = ['USDT','BTC','ETH','BNB','SOL','XRP','ADA','DOGE','TON','TRX','DOT'];
        baseSel.innerHTML  = fallback.map(b => `<option value="${b}">${b}</option>`).join('');
        quoteSel.innerHTML = fallback.map(q => `<option value="${q}">${q}</option>`).join('');
        quoteSel.value = 'USDT';
    }
}

/* ========== UI: карточки и разметка ========== */
function scenarioTitle(s) {
    const t = dict[currentLang];
    if (s === 'best_single') return t.best_single;
    if (s === 'equal_split') return t.equal_split;
    if (s === 'optimal')     return t.optimal;
    return s || '';
}

function buildSummaryHTML(j){
    const t = dict[currentLang];

    const baseU  = String(j.base  || '').toUpperCase();
    const quoteU = String(j.quote || '').toUpperCase();

    const legs = Array.isArray(j.legs) ? j.legs : [];
    // Важно: "получено" берём из server-side поля `generated`
    const received = Number(j.generated || 0);

    const bothNotUsdt = (baseU !== 'USDT' && quoteU !== 'USDT'); // маршрут монета→монета
    const isUsdtQuote = (quoteU === 'USDT');

    // Средняя цена:
    // - если quote=USDT → показываем USDT за 1 BASE
    // - если монета→монета → показываем BASE за 1 QUOTE (эффективная)
    const avgVal   = Number(j.vwap || 0);
    const avgNum   = isUsdtQuote ? priceUSDT(avgVal) : qtyBASE(avgVal);
    const avgUnits = isUsdtQuote ? `${t.usdt} ${t.per} ${j.base || ''}` : `${baseU}/${quoteU}`;

    // Сколько "затрачено": берём фактический totalCost от сервера.
    // Единицы:
    //   - USDT→монета: показываем в USDT
    //   - монета→монета: показываем в валюте оплаты (quote)
    const spentVal   = Number(j.totalCost ?? j.amount ?? 0);
    const spendNum   = bothNotUsdt ? qtyCOINTerse(spentVal) : moneyUSDT(spentVal);
    const spendUnits = bothNotUsdt ? (j.quote || '') : t.usdt;

    // Не израсходовано (в тех же единицах, что и "Затраты")
    const unspentVal = Number(j.unspent || 0);
    const unspentStr = bothNotUsdt ? `${qtyCOINTerse(unspentVal)} ${j.quote || ''}`
        : `${moneyUSDT(unspentVal)} ${t.usdt}`;

    const unspentBlock = (unspentVal > 0.0000001)
        ? `<div><strong>${t.unspent}:</strong> ${unspentStr}</div>` : '';

    // Стоимость без комиссий и итого к оплате — в тех же единицах, что и "Затраты"
    const assetsNoFeesVal = Number(j.totalCost || 0) - Number(j.totalFees || 0);
    const assetsNoFeesNum = bothNotUsdt ? qtyCOINTerse(assetsNoFeesVal) : moneyUSDT(assetsNoFeesVal);
    const totalToPayNum   = bothNotUsdt ? qtyCOINTerse(Number(j.totalCost || 0))
        : moneyUSDT(Number(j.totalCost || 0));
    const unitStr = spendUnits;

    // Описание сценария
    const descriptions = {
        best_single: currentLang === 'ru'
            ? 'Вся сумма уходит на одну биржу с наилучшей ценой'
            : 'All funds go to the single exchange with the best price',
        equal_split: currentLang === 'ru'
            ? 'Сумма делится равными частями между всеми биржами'
            : 'Funds are split equally across all exchanges',
        optimal: currentLang === 'ru'
            ? 'Сумма распределяется оптимально между биржами для лучшей цены'
            : 'Funds are distributed optimally across exchanges for best execution',
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
        <div><strong>${t.spendLabel}:</strong> ${spendNum} ${spendUnits}</div>
        <div><strong>${t.avgPrice}:</strong> ${avgNum} ${avgUnits}</div>
        <div><strong>${t.assetsNoFees}:</strong> ${assetsNoFeesNum} ${unitStr}</div>
        <div><strong>${t.totalToPay}:</strong> ${totalToPayNum} ${unitStr}</div>
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
    <table class="grid-table">
      <thead>
        <tr>
          <th>${t.exchange}</th>
          <th class="num">${t.amountCol} (${j.base || '-'}/${j.quote || '-'})</th>
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

/* ========== Инициализация ========== */
document.addEventListener('DOMContentLoaded', () => {
    // Год в футере
    const y = $('footer-year');
    if (y) y.textContent = new Date().getFullYear();

    // Язык
    setLang(currentLang);
    $('btn-en')?.addEventListener('click', () => setLang('en'));
    $('btn-ru')?.addEventListener('click', () => setLang('ru'));

    // Форматирование инпута суммы: "точки" как разделители тысяч
    const amountInput = $('amount');
    if (amountInput) {
        amountInput.addEventListener('input', (e) => {
            const caret = e.target.selectionStart;
            const digits = e.target.value.replace(/\./g, '');
            e.target.value = formatThousandsDots(digits);
            // карет можно не восстанавливать идеально — достаточно не прыгать в начало
            e.target.selectionStart = e.target.selectionEnd = Math.min(caret + 1, e.target.value.length);
        });
    }

    // Первичные запросы
    checkHealth();
    loadSymbols();

    // Сабмит формы
    const form = $('plan-form');
    const cmp  = $('comparisons'); // контейнер для 3 сценариев
    const btn  = $('calc-btn');

    form?.addEventListener('submit', async (e) => {
        e.preventDefault();
        if (!btn || !cmp) return;
        btn.disabled = true;

        const base  = $('base')?.value?.trim().toUpperCase()  || 'BTC';
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
                    body: reqBody(scenario),
                }).then(async r => {
                    const text = await r.text();
                    if (!r.ok) throw new Error(text || `HTTP ${r.status}`);
                    const j = JSON.parse(text);
                    if (j && j.error) throw new Error(j.error);
                    return j;
                });

            const results = await Promise.all(scenarios.map(fetchOne));

            // заголовок + три панели
            const title = `<h2 class="muted" style="margin:8px 0 0 2px;">${dict[currentLang].resultsFor}: ${base}/${quote}</h2>`;
            cmp.innerHTML = title + results.map(buildScenarioPanel).join('');
        } catch (err) {
            cmp.innerHTML = `<section class="card"><h2>Error</h2><pre style="white-space:pre-wrap">${String(err)}</pre></section>`;
        } finally {
            btn.disabled = false;
        }
    });
});

