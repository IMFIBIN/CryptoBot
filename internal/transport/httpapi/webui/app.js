const $ = (id) => document.getElementById(id);

const scenarioTitle = (code) => ({
    best_single: 'Best single',
    equal_split: 'Equal split',
    optimal:     'Optimal',
}[code] || code);

const moneyUSDT = (n) => Number(n).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const qtyBASE   = (n) => Number(n).toLocaleString('en-US', { minimumFractionDigits: 6, maximumFractionDigits: 6 });
const priceUSDT = (n) => Number(n).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

async function checkHealth() {
    try {
        const r = await fetch('/api/health', { cache: 'no-store' });
        const j = await r.json();
        $('health').textContent = j.status === 'ok' ? 'Server: OK' : 'Server: error';
    } catch { $('health').textContent = 'Server unreachable'; }
}

async function loadSymbols() {
    try {
        const r = await fetch('/api/symbols', { cache: 'no-store' });
        const j = await r.json();
        fillSelect($('base'), j.bases, 'BTC');
    } catch {
        fillSelect($('base'), ['BTC','ETH','SOL'], 'BTC');
    }
}

function fillSelect(sel, items, def) {
    sel.innerHTML = items.map(x => `<option value="${x}">${x}</option>`).join('');
    if (def && items.includes(def)) sel.value = def;
}

checkHealth(); loadSymbols();

// ---- Spend only integers ----
const toIntDollars = (x) => {
    const digits = String(x).replace(/\D+/g, '');
    const n = digits === '' ? 0 : parseInt(digits, 10);
    return Math.max(1, n);
};
$('amount').addEventListener('input', () => {
    $('amount').value = String(toIntDollars($('amount').value));
});

// helpers
const sumAmount = (legs) => legs.reduce((s, l) => s + Number(l.amount || 0), 0);

$('plan-form').addEventListener('submit', async (e) => {
    e.preventDefault();

    if ($('base').value === 'USDT') { alert('Base must differ from USDT'); return; }

    const amount = toIntDollars($('amount').value);
    $('amount').value = String(amount);

    const res  = $('result');
    const legs = $('legs');
    res.classList.remove('hidden'); res.innerHTML = '<h2>Calculating…</h2>';
    legs.classList.add('hidden');   legs.innerHTML = '';

    const btn = $('calc-btn'); btn.disabled = true;

    const payload = {
        base: $('base').value,
        quote: 'USDT',
        amount: amount,
        depth: parseInt($('depth').value, 10),
        scenario: $('scenario').value,
    };

    try {
        const r = await fetch('/api/plan', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' },
            body: JSON.stringify(payload)
        });
        const j = await r.json();
        if (!r.ok) throw new Error(j.error || 'API error');

        const gotBase = sumAmount(j.legs);
        const totalPay = Number(j.amount) + Number(j.totalFees);

        res.innerHTML = `
      <h2>Summary</h2>
      <div class="grid">
        <div><b>Pair:</b> ${j.base}/${j.quote}</div>
        <div><b>Scenario:</b> ${scenarioTitle(j.scenario)}</div>
        <div><b>Spend:</b> ${moneyUSDT(j.amount)} USDT</div>
        <div><b>Receive:</b> ${qtyBASE(gotBase)} ${j.base}</div>
        <div><b>Average execution price:</b> ${priceUSDT(j.vwap)} USDT per 1 ${j.base}</div>
        <div><b>Assets cost (no fees):</b> ${moneyUSDT(j.totalCost)} USDT</div>
        <div><b>Unspent (not used due to orderbook depth):</b> ${moneyUSDT(j.unspent)} USDT</div>
        <div><b>Fees:</b> ${moneyUSDT(j.totalFees)} USDT</div>
        <div><b>Total to pay:</b> ${moneyUSDT(totalPay)} USDT</div>
        <div><b>Current time:</b> ${j.generatedAt}</div>
      </div>`;

        const rows = j.legs.map(l => ({
            ex: l.exchange,
            amount: Number(l.amount),
            price: Number(l.price),
            fee: Number(l.fee),
        }));

        const scenario = $('scenario').value;
        let bestEx = null, worstEx = null;

        if (scenario === 'best_single' || scenario === 'equal_split') {
            // среди строк (для equal_split — только там где amount > 0)
            const candidates = scenario === 'equal_split' ? rows.filter(r => r.amount > 0) : rows;
            let bestPrice = Infinity, worstPrice = -Infinity;
            for (const r of candidates) {
                if (r.price < bestPrice) { bestPrice = r.price; bestEx = r.ex; }
                if (r.price > worstPrice) { worstPrice = r.price; worstEx = r.ex; }
            }
        }

        const rowHtml = rows.map(r => {
            let cls = '';
            if (scenario === 'best_single' || scenario === 'equal_split') {
                if (r.ex === bestEx) cls = 'best-row';
                else if (r.ex === worstEx) cls = 'worst-row';
            }
            return `
        <tr class="${cls}">
          <td>${r.ex}</td>
          <td class="num">${qtyBASE(r.amount)}</td>
          <td class="num">${priceUSDT(r.price)}</td>
          <td class="num">${moneyUSDT(r.fee)}</td>
        </tr>`;
        }).join('');

        legs.classList.remove('hidden');
        legs.innerHTML = `<h2>Allocation</h2>
      <table class="data">
        <colgroup><col class="col-exchange" /><col class="col-num" /><col class="col-num" /><col class="col-num" /></colgroup>
        <thead>
          <tr>
            <th>Exchange</th>
            <th class="num">Amount (${j.base})</th>
            <th class="num">Price (USDT/${j.base})</th>
            <th class="num">Fee (USDT)</th>
          </tr>
        </thead>
        <tbody>${rowHtml}</tbody>
      </table>`;
    } catch (err) {
        res.innerHTML = `<h2>Error</h2><pre>${String(err)}</pre>`;
    } finally {
        btn.disabled = false;
    }
});
