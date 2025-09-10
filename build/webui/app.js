// === i18n (EN/RU) ===
const I18N = {
    en: {
        buy: 'Buy',
        pay: 'Pay',
        spend: 'Spend',
        depth: 'Orderbook depth',
        scenario: 'Scenario',
        calculate: 'Calculate',
        scenarioOptions: {
            best_single: 'Best single',
            equal_split: 'Equal split',
            optimal: 'Optimal',
        },
        checkingServer: 'Checking server…',
        serverOK: 'Server is up',
        serverFail: 'Server is not responding',
        resultTitle: 'Plan result',
        legsTitle: 'Execution legs',

        // result card field labels
        fBase: 'Base',
        fQuote: 'Quote',
        fAmount: 'Amount',
        fScenario: 'Scenario',
        fVWAP: 'VWAP',
        fTotalCost: 'Total cost',
        fTotalFees: 'Total fees',
        fUnspent: 'Unspent',
        fGenerated: 'Generated',

        // table headers
        thExchange: 'Exchange',
        thAmount: 'Amount',
        thPrice: 'Price',
        thFee: 'Fee',

        errBadAmount: 'Enter a valid amount',
        errBadDepth: 'Enter a valid depth',
        errRequest: 'Request failed',
    },
    ru: {
        buy: 'Купить',
        pay: 'Платить',
        spend: 'Потратить',
        depth: 'Глубина стакана',
        scenario: 'Сценарий',
        calculate: 'Рассчитать',
        scenarioOptions: {
            best_single: 'Лучшая одна биржа',
            equal_split: 'Равные доли',
            optimal: 'Оптимально',
        },
        checkingServer: 'Проверяем сервер…',
        serverOK: 'Сервер работает',
        serverFail: 'Сервер не отвечает',
        resultTitle: 'Результат плана',
        legsTitle: 'Ноги исполнения',

        // result card field labels
        fBase: 'Базовая',
        fQuote: 'Котируемая',
        fAmount: 'Сумма',
        fScenario: 'Сценарий',
        fVWAP: 'Средняя цена (VWAP)',
        fTotalCost: 'Итого потрачено',
        fTotalFees: 'Комиссии',
        fUnspent: 'Остаток',
        fGenerated: 'Сгенерировано',

        // table headers
        thExchange: 'Биржа',
        thAmount: 'Количество',
        thPrice: 'Цена',
        thFee: 'Комиссия',

        errBadAmount: 'Введите корректную сумму',
        errBadDepth: 'Введите корректную глубину',
        errRequest: 'Ошибка запроса',
    }
};

let lastResponse = null; // для перерисовки после смены языка

function getSavedLang() {
    const ls = localStorage.getItem('lang');
    if (ls === 'ru' || ls === 'en') return ls;
    return (navigator.language || '').toLowerCase().startsWith('ru') ? 'ru' : 'en';
}

function setLang(lang) {
    const dict = I18N[lang] || I18N.en;
    document.documentElement.lang = lang;

    document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.getAttribute('data-i18n');
        if (dict[key]) el.textContent = dict[key];
    });

    const sel = document.getElementById('scenario');
    if (sel) {
        [...sel.options].forEach(o => {
            const v = o.value;
            if (dict.scenarioOptions[v]) o.textContent = dict.scenarioOptions[v];
        });
    }

    const btn = document.getElementById('langBtn');
    if (btn) btn.textContent = (lang === 'en') ? 'RU' : 'EN';

    const health = document.getElementById('health');
    if (health && (health.dataset.state === 'checking')) {
        health.textContent = dict.checkingServer;
    }

    // Перерисовать уже полученный результат на новом языке
    if (lastResponse) {
        renderResult(lastResponse);
    }

    localStorage.setItem('lang', lang);
}

function toggleLang() {
    const cur = document.documentElement.lang || getSavedLang();
    setLang(cur === 'ru' ? 'en' : 'ru');
}

// === утилиты ===
const byId = (id) => document.getElementById(id);

function currentLocale() {
    const l = document.documentElement.lang || 'en';
    return l === 'ru' ? 'ru-RU' : 'en-US';
}

function parseNumber(str) {
    if (typeof str !== 'string') return NaN;
    const s = str.replace(/\s+/g, '').replace(/,/g, '.');
    return Number(s);
}

function fmt2(x) {
    if (!isFinite(x)) return '—';
    return new Intl.NumberFormat(currentLocale(), { maximumFractionDigits: 2 }).format(x);
}

function fmt8(x) {
    if (!isFinite(x)) return '—';
    return new Intl.NumberFormat(currentLocale(), { maximumFractionDigits: 8 }).format(x);
}

async function apiGet(url) {
    const r = await fetch(url, { credentials: 'same-origin' });
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
    return r.json();
}

async function apiPost(url, body) {
    const r = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'same-origin',
        body: JSON.stringify(body),
    });
    if (!r.ok) {
        const text = await r.text().catch(() => '');
        throw new Error(`HTTP ${r.status} ${text}`);
    }
    return r.json();
}

// === логика страницы ===
async function checkHealth() {
    const health = byId('health');
    const dict = I18N[document.documentElement.lang] || I18N.en;
    health.dataset.state = 'checking';
    health.textContent = dict.checkingServer;

    try {
        await apiGet('/api/health');
        health.textContent = dict.serverOK;
        health.classList.remove('bad');
    } catch (e) {
        health.textContent = dict.serverFail;
        health.classList.add('bad');
    } finally {
        delete health.dataset.state;
    }
}

function fillSelectWithCoins(select, coins) {
    select.innerHTML = '';
    for (const s of coins) {
        const opt = document.createElement('option');
        opt.value = s;
        opt.textContent = s;
        select.appendChild(opt);
    }
}

async function loadSymbols() {
    const baseSel = byId('base');
    const paySel = byId('pay');

    try {
        const data = await apiGet('/api/symbols');
        let bases = [];
        if (Array.isArray(data)) bases = data;
        else if (Array.isArray(data.bases)) bases = data.bases;
        else if (Array.isArray(data.base)) bases = data.base;
        else if (Array.isArray(data.symbols)) bases = data.symbols;

        if (!bases || bases.length === 0) {
            bases = ['BTC', 'ETH', 'BNB', 'ADA', 'SOL']; // fallback
        }

        const coins = Array.from(new Set(['USDT', ...bases])); // добавим USDT всегда
        fillSelectWithCoins(baseSel, coins);
        fillSelectWithCoins(paySel, coins);

        // по умолчанию: покупаем BTC за USDT
        baseSel.value = 'BTC';
        paySel.value = 'USDT';
    } catch (e) {
        const coins = ['USDT', 'BTC', 'ETH', 'BNB', 'ADA', 'SOL'];
        fillSelectWithCoins(baseSel, coins);
        fillSelectWithCoins(paySel, coins);
        baseSel.value = 'BTC';
        paySel.value = 'USDT';
    }
}

function renderResult(resp) {
    lastResponse = resp; // запомним для смены языка

    const result = byId('result');
    const legs = byId('legs');
    const dict = I18N[document.documentElement.lang] || I18N.en;

    result.classList.remove('hidden');
    legs.classList.remove('hidden');

    const scenarioLabel = dict.scenarioOptions[resp.scenario] || resp.scenario;

    result.innerHTML = `
    <h2>${dict.resultTitle}</h2>
    <div class="kv">
      <div><span>${dict.fBase}</span><b>${resp.base}</b></div>
      <div><span>${dict.fQuote}</span><b>${resp.quote}</b></div>
      <div><span>${dict.fAmount}</span><b>${fmt2(resp.amount)}</b></div>
      <div><span>${dict.fScenario}</span><b>${scenarioLabel}</b></div>
      <div><span>${dict.fVWAP}</span><b>${fmt8(resp.vwap)}</b></div>
      <div><span>${dict.fTotalCost}</span><b>${fmt2(resp.totalCost)}</b></div>
      <div><span>${dict.fTotalFees}</span><b>${fmt2(resp.totalFees)}</b></div>
      <div><span>${dict.fUnspent}</span><b>${fmt2(resp.unspent)}</b></div>
      <div><span>${dict.fGenerated}</span><b>${resp.generatedAt || ''}</b></div>
    </div>
  `;

    const rows = (resp.legs || []).map(l => `
    <tr>
      <td>${l.exchange}</td>
      <td class="num">${fmt8(l.amount)}</td>
      <td class="num">${fmt8(l.price)}</td>
      <td class="num">${fmt2(l.fee)}</td>
    </tr>
  `).join('');

    legs.innerHTML = `
    <h2>${dict.legsTitle}</h2>
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>${dict.thExchange}</th>
            <th>${dict.thAmount}</th>
            <th>${dict.thPrice}</th>
            <th>${dict.thFee}</th>
          </tr>
        </thead>
        <tbody>${rows}</tbody>
      </table>
    </div>
  `;
}

function showError(err) {
    lastResponse = null;

    const result = byId('result');
    const legs = byId('legs');
    const dict = I18N[document.documentElement.lang] || I18N.en;

    result.classList.remove('hidden');
    legs.classList.add('hidden');

    result.innerHTML = `
    <h2>${dict.resultTitle}</h2>
    <div class="error">${dict.errRequest}: ${String(err && err.message || err)}</div>
  `;
}

async function onSubmit(e) {
    e.preventDefault();
    const dict = I18N[document.documentElement.lang] || I18N.en;

    const buy = byId('base').value; // что хотим получить
    const pay = byId('pay').value;  // чем платим
    const amount = parseNumber(byId('amount').value);
    const depth = parseInt(byId('depth').value, 10);
    const scenario = byId('scenario').value;

    if (!isFinite(amount) || amount <= 0) {
        alert(dict.errBadAmount);
        return;
    }
    if (!Number.isInteger(depth) || depth <= 0) {
        alert(dict.errBadDepth);
        return;
    }
    if (buy === pay) {
        alert('Buy and Pay must be different');
        return;
    }

    // Всегда шлём как есть: base=Buy, quote=Pay, amount — в единицах Pay
    const payload = { base: buy, quote: pay, amount, depth, scenario };

    byId('calc-btn').disabled = true;

    try {
        const resp = await apiPost('/api/plan', payload);
        renderResult(resp);
    } catch (err) {
        showError(err);
    } finally {
        byId('calc-btn').disabled = false;
    }
}

// === bootstrap ===
document.addEventListener('DOMContentLoaded', () => {
    setLang(getSavedLang());
    const btn = byId('langBtn');
    if (btn) btn.addEventListener('click', toggleLang);

    checkHealth();
    loadSymbols();

    byId('plan-form').addEventListener('submit', onSubmit);
});
