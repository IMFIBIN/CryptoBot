const $ = (id) => document.getElementById(id);

async function checkHealth() {
    try {
        const r = await fetch('/api/health', {cache: 'no-store'});
        const j = await r.json();
        $('health').textContent = j.status === 'ok' ? 'Сервер: OK' : 'Сервер: ошибка';
    } catch (e) { $('health').textContent = 'Сервер недоступен'; }
}

async function loadSymbols() {
    try {
        const r = await fetch('/api/symbols', {cache: 'no-store'});
        const j = await r.json();
        fillSelect($('base'),  j.bases,  'ETH');
        fillSelect($('quote'), j.quotes, 'USDT');
    } catch (e) {
        fillSelect($('base'),  ['BTC','ETH','BNB']);
        fillSelect($('quote'), ['USDT']);
    }
}

function fillSelect(sel, items, def) {
    sel.innerHTML = items.map(x => `<option value="${x}">${x}</option>`).join('');
    if (def && items.includes(def)) sel.value = def;
}

function nowTick() { $('now').textContent = new Date().toLocaleString(); }
setInterval(nowTick, 1000); nowTick();
checkHealth();
loadSymbols();

function fmt(n, d=6) { return Number(n).toFixed(d); }

$('plan-form').addEventListener('submit', async (e) => {
    e.preventDefault();

    // визуальное обновление гарантированно: сразу очищаем
    const res  = $('result');
    const legs = $('legs');
    res.classList.remove('hidden'); res.innerHTML = '<h2>Расчёт...</h2>';
    legs.classList.add('hidden');   legs.innerHTML = '';

    const btn = $('calc-btn');
    btn.disabled = true;

    const payload = {
        base: $('base').value,
        quote: $('quote').value,
        amount: parseFloat($('amount').value),
        depth: parseInt($('depth').value, 10),
        scenario: $('scenario').value
    };

    try {
        const r = await fetch('/api/plan', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' },
            body: JSON.stringify(payload)
        });
        const j = await r.json();
        if (!r.ok) throw new Error(j.error || 'API error');

        res.innerHTML = `
      <h2>Итог</h2>
      <div class="grid">
        <div><b>Пара:</b> ${j.base}/${j.quote}</div>
        <div><b>Сценарий:</b> ${j.scenario}</div>
        <div><b>Сумма:</b> ${fmt(j.amount, 4)}</div>
        <div><b>VWAP:</b> ${fmt(j.vwap, 6)}</div>
        <div><b>Итоговая стоимость:</b> ${fmt(j.totalCost, 6)}</div>
        <div><b>Комиссии:</b> ${fmt(j.totalFees, 6)}</div>
        <div><b>Сгенерировано:</b> ${j.generatedAt}</div>
      </div>`;

        legs.classList.remove('hidden');
        legs.innerHTML = `<h2>Распределение</h2>
      <table>
        <thead><tr><th>Биржа</th><th>Объём</th><th>Цена</th><th>Комиссия</th></tr></thead>
        <tbody>
          ${j.legs.map(l => `
            <tr>
              <td>${l.exchange}</td>
              <td class="num">${fmt(l.amount, 4)}</td>
              <td class="num">${fmt(l.price, 6)}</td>
              <td class="num">${fmt(l.fee, 6)}</td>
            </tr>`).join('')}
        </tbody>
      </table>`;
    } catch (err) {
        res.innerHTML = `<h2>Ошибка</h2><pre>${String(err)}</pre>`;
    } finally {
        btn.disabled = false;
    }
});
