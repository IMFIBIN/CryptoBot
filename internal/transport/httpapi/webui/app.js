// === i18n (EN/RU) ===
const I18N = {
    en: {
        buy: 'Buy',
        pay: 'Pay',
        spend: 'Spend (USDT)',
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
        errBadAmount: 'Enter a valid amount',
        errBadDepth: 'Enter a valid depth',
        errRequest: 'Request failed',
    },
    ru: {
        buy: 'Купить',
        pay: 'Платить',
        spend: 'Потратить (USDT)',
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
        errBadAmount: 'Введите корректную сумму',
        errBadDepth: 'Введите корректную глубину',
        errRequest: 'Ошибка запроса',
    }
};

function getSavedLang() {
    const ls = localStorage.getItem('lang');
    if (ls === 'ru' || ls === 'en') return ls;
    return (navigator.language || '').toLowerCase().startsWith('ru') ? 'ru' : 'en';
}

function setLang(lang) {
    const dict = I18N[lang] || I18N.en;
    document.documentElement.lang = lang;

    // статические подписи по data-i18n
    document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.getAttribute('data-i18n');
        if (dict[key]) el.textContent = dict[key];
    });

    // подписи опций сценариев (value не меняем!)
    const sel = document.getElementById('scenario');
    if (sel) {
        [...sel.options].forEach(o => {
            const v = o.value;
            if (dict.scenarioOptions[v]) o.textContent = dict.scenarioOptions[v];
        });
    }

    // кнопка языка
    const btn = document.getElementById('langBtn');
    if (btn) btn.textContent = (lang === 'en') ? 'RU' : 'EN';

    // подпись статуса сервера если ещё не перезаписана
    const health = document.getElementById('health');
    if (health && (health.dataset.state === 'checking')) {
        health.textContent = dict.checkingServer;
    }

    localStorage.setItem('lang', lang);
}

function toggleLang() {
    const cur = document.documentElement.lang || getSavedLang();
    setLang(cur === 'ru' ? 'en' : 'ru');
}

// === утилиты ===
const $ = (sel) => document.querySelector(sel);
const byId = (id) => document.getElementById(id);

function parseNumber(str) {
    if (typeof str !== 'string') return NaN;
    // поддержим "1 000 000" и "1,000,000.5" и "1 000 000,5"
    const s = str.replace(/\s+/g, '').replace(/,/g, '.');
    return Number(s);
}

function fmt2(x) {
    if (!isFinite(x)) return '—';
    return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(x);
}

function fmt8(x) {
    if (!isFinite(x)) return '—';
    return new Intl.NumberFormat(undefined, { maximumFractionDigits: 8 }).format(x);
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

async function loadSymbols() {
    const baseSel = byId('base');

    try {
        const data = await apiGet('/api/symbols');
        // поддержим разные формы ответа:
        // { bases: [...] } | { base: [...] } | { symbols: [...] } | [...]
        let bases = [];
        if (Array.isArray(data)) bases = data;
        else if (Array.isArray(data.bases)) bases = data.bases;
        else if (Array.isArray(data.base)) bases = data.base;
        else if (Array.isArray(data.symbols)) bases = data.symbols;

        if (!bases || bases.length === 0) {
            bases = ['BTC', 'ETH', 'BNB', 'ADA', 'SOL']; // запасной вариант
        }

        baseSel.innerHTML = '';
        for (const s of bases) {
            const opt = document.createElement('option');
            opt.value = s;
            opt.textContent = s;
            baseSel.appendChild(opt);
        }
    } catch (e) {
        // в крайнем случае — статический набор
        baseSel.innerHTML = '';
        ['BTC', 'ETH', 'BNB', 'ADA', 'SOL'].forEach(s => {
            const opt = document.createElement('option');
            opt.value = opt.textContent = s;
            baseSel.appendChild(opt);
        });
    }
}

function renderResult(resp) {
    // ВАЖНО: бэкенд отдаёт lowerCamelCase / kebab? — тут используем нижний регистр:
    const result = byId('result');
    const legs = byId('legs');
    const dict = I18N[document.documentElement.lang] || I18N.en;

    result.classList.remove('hidden');
    legs.classList.remove('hidden');

    const scenarioLabel = dict.scenarioOptions[resp.scenario] || resp.scenario;

    result.innerHTML = `
    <h2>${dict.resultTitle}</h2>
    <div class="kv">
      <div><span>Base</span><b>${resp.base}</b></div>
      <div><span>Quote</span><b>${resp.quote}</b></div>
      <div><span>Amount</span><b>${fmt2(resp.amount)}</b></div>
      <div><span>Scenario</span><b>${scenarioLabel}</b></div>
      <div><span>VWAP</span><b>${fmt8(resp.vwap)}</b></div>
      <div><span>Total cost</span><b>${fmt2(resp.totalCost)}</b></div>
      <div><span>Total fees</span><b>${fmt2(resp.totalFees)}</b></div>
      <div><span>Unspent</span><b>${fmt2(resp.unspent)}</b></div>
      <div><span>Generated</span><b>${resp.generatedAt || ''}</b></div>
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
            <th>Exchange</th>
            <th>Amount</th>
            <th>Price</th>
            <th>Fee</th>
          </tr>
        </thead>
        <tbody>${rows}</tbody>
      </table>
    </div>
  `;
}

function showError(err) {
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

    const base = byId('base').value || 'BTC';
    const quote = 'USDT';
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

    byId('calc-btn').disabled = true;

    try {
        const payload = { base, quote, amount, depth, scenario };
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
    // язык
    setLang(getSavedLang());
    const btn = byId('langBtn');
    if (btn) btn.addEventListener('click', toggleLang);

    // данные
    checkHealth();
    loadSymbols();

    // форма
    byId('plan-form').addEventListener('submit', onSubmit);
});
