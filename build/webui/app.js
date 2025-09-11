// === i18n (EN/RU) ===
const I18N = {
    en: {
        buy: 'Buy',
        pay: 'Pay',
        spend: 'Spend',
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
        summaryTitle: 'Summary',
        allocationTitle: 'Allocation',
        intro: {
            best_single: 'All funds go to the single exchange with the best price',
            equal_split: 'Funds are split equally across all exchanges',
            optimal: 'Funds are allocated optimally across exchanges',
        },
        pair: 'Pair',
        receive: 'Receive',
        currentTime: 'Current time',
        spendLabel: 'Spend',
        avgExecPrice: 'Average execution price',
        assetsCost: 'Assets cost',
        totalToPay: 'Total to pay',
        fUnspent: 'Unspent',
        fGenerated: 'Generated',
        thExchange: 'Exchange',
        thAmountUnit: 'Amount ({unit})',
        thPriceUnit: 'Price ({quote}/{base})',
        resultsFor: 'Results for',
        errBadAmount: 'Enter a valid amount',
        errRequest: 'Request failed',
    },
    ru: {
        buy: 'Купить',
        pay: 'Отдаёте',
        spend: 'Сумма',
        calculate: 'Рассчитать',
        scenarioOptions: {
            best_single: 'Самая выгодная биржа',
            equal_split: 'Равное распределение средств по биржам',
            optimal: 'Лучшее распределение средств',
        },
        checkingServer: 'Проверяем сервер…',
        serverOK: 'Сервер работает',
        serverFail: 'Сервер не отвечает',
        resultTitle: 'Результат',
        legsTitle: 'Сделки',
        summaryTitle: 'Сводка',
        allocationTitle: 'Распределение',
        intro: {
            best_single: 'Все средства на одну биржу с лучшей ценой',
            equal_split: 'Средства поровну распределены по биржам',
            optimal: 'Средства распределены оптимально по биржам',
        },
        pair: 'Пара',
        receive: 'Получите',
        currentTime: 'Текущее время',
        spendLabel: 'Потратить',
        avgExecPrice: 'Средняя цена исполнения',
        assetsCost: 'Стоимость актива',
        totalToPay: 'Итого к оплате',
        fUnspent: 'Остаток',
        fGenerated: 'Сгенерировано',
        thExchange: 'Биржа',
        thAmountUnit: 'Количество ({unit})',
        thPriceUnit: 'Цена ({quote}/{base})',
        resultsFor: 'Результаты для',
        errBadAmount: 'Введите корректную сумму',
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

        const coins = Array.from(new Set(['USDT', ...bases]));
        fillSelectWithCoins(baseSel, coins);
        fillSelectWithCoins(paySel, coins);

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
    lastResponse = resp;
    const result = byId('result');
    const legs = byId('legs');
    const dict = I18N[document.documentElement.lang] || I18N.en;

    result.classList.remove('hidden');
    legs.classList.remove('hidden');

    const scenarioLabel = dict.scenarioOptions[resp.scenario] || resp.scenario;
    const introText = (dict.intro && dict.intro[resp.scenario]) ? dict.intro[resp.scenario] : '';

    const thAmountUnit = (dict.thAmountUnit || '{unit}').replace('{unit}', resp.base);
    const thPriceUnit = (dict.thPriceUnit || '{quote}/{base}')
        .replace('{quote}', resp.quote)
        .replace('{base}', resp.base);

    result.innerHTML = `
    <h2>${dict.summaryTitle} — ${scenarioLabel}</h2>
    ${introText ? `<div class="muted">${introText}</div>` : ''}
    <div class="kv">
      <div><span>${dict.pair}</span><b>${resp.base}/${resp.quote}</b></div>
      <div><span>${dict.spendLabel}</span><b>${fmt2(resp.amount)} ${resp.quote}</b></div>
      <div><span>${dict.receive}</span><b>${fmt8(resp.totalQty || 0)} ${resp.base}</b></div>
      <div><span>${dict.avgExecPrice}</span><b>${fmt8(resp.vwap)} ${resp.quote}/${resp.base}</b></div>
      <div><span>${dict.assetsCost}</span><b>${fmt2(resp.totalCost)} ${resp.quote}</b></div>
      <div><span>${dict.totalToPay}</span><b>${fmt2(resp.totalCost)} ${resp.quote}</b></div>
      <div><span>${dict.fUnspent}</span><b>${fmt2(resp.unspent)} ${resp.quote}</b></div>
      <div><span>${dict.currentTime}</span><b>${resp.generatedAt || ''}</b></div>
    </div>
  `;

    const rows = (resp.legs || []).map(l => `
    <tr>
      <td>${l.exchange}</td>
      <td class="num">${fmt8(l.amount)}</td>
      <td class="num">${fmt8(l.price)}</td>
    </tr>
  `).join('');

    legs.innerHTML = `
    <h2>${dict.allocationTitle}</h2>
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>${dict.thExchange}</th>
            <th>${thAmountUnit}</th>
            <th>${thPriceUnit}</th>
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

    const buy = byId('base').value;
    const pay = byId('pay').value;
    const amount = parseNumber(byId('amount').value);
    const scenario = byId('scenario').value;

    if (!isFinite(amount) || amount <= 0) {
        alert(dict.errBadAmount);
        return;
    }
    if (buy === pay) {
        alert('Buy and Pay must be different');
        return;
    }

    const payload = { base: buy, quote: pay, amount, scenario };

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

document.addEventListener('DOMContentLoaded', () => {
    setLang(getSavedLang());
    const btn = byId('langBtn');
    if (btn) btn.addEventListener('click', toggleLang);

    checkHealth();
    loadSymbols();
    byId('plan-form').addEventListener('submit', onSubmit);
});
