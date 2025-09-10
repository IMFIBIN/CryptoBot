const $ = (id) => document.getElementById(id);

const scenarioTitle = (code) => ({
    best_single: 'Best single',
    equal_split: 'Equal split',
    optimal:     'Optimal',
}[code] || code);

const moneyUSDT = (n) => Number(n).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const qtyBASE   = (n) => Number(n).toLocaleString('en-US', { minimumFractionDigits: 6, maximumFractionDigits: 6 });
const priceUSDT = (n) => Number(n).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

// безопасные суммирования
const sumAmount = (legs) => (Array.isArray(legs) ? legs : [])
    .reduce((s, l) => s + Number((l && l.amount) || 0), 0);
const sumFees = (legs) => (Array.isArray(legs) ? legs : [])
    .reduce((s, l) => s + Number((l && l.fee) || 0), 0);

// health
async function checkHealth() {
    const node = $('health');
    try {
        const r = await fetch('/api/health', { cache: 'no-store' });
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        const j = await r.json();
        node.textContent = j?.status ? `Server: OK` : 'Server: OK';
        node.classList.remove('muted', 'bad');
    } catch {
        node.textContent = 'Server is unreachable';
        node.classList.add('bad');
    }
}

// загрузка символов
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
        // fallback если /api/symbols не отвечает
        baseSel.innerHTML  = ['BTC','ETH','BNB','SOL','XRP','ADA','DOGE','TON','TRX','DOT']
            .map(b => `<option value="${b}">${b}</option>`).join('');
        quoteSel.value = 'USDT';
    }
}

// Summary
function renderSummary(card, j) {
    const legs = Array.isArray(j.legs) ? j.legs : [];
    const received = sumAmount(legs);
    const assetsNoFees = Number(j.totalCost || 0) - Number(j.totalFees || 0);

    card.classList.remove('hidden');
    card.innerHTML = `
      <h2>Summary</h2>
      <div class="grid-2">
        <div>
          <div><strong>Pair:</strong> ${j.base || '-'} / ${j.quote || '-'}</div>
          <div><strong>Receive:</strong> ${qtyBASE(received)} ${j.base || ''}</div>
          <div><strong>Unspent (not used due to orderbook depth):</strong> ${moneyUSDT(j.unspent || 0)} ${j.quote || 'USDT'}</div>
          <div><strong>Current time:</strong> ${j.generatedAt || ''}</div>
        </div>
        <div>
          <div><strong>Scenario:</strong> ${scenarioTitle(j.scenario || '')}</div>
          <div><strong>Spend:</strong> ${moneyUSDT(j.amount || 0)} ${j.quote || 'USDT'}</div>
          <div><strong>Average execution price:</strong> ${priceUSDT(j.vwap || 0)} USDT per 1 ${j.base || ''}</div>
          <div><strong>Assets cost (no fees):</strong> ${moneyUSDT(assetsNoFees)} USDT</div>
          <div><strong>Fees:</strong> ${moneyUSDT(j.totalFees || 0)} USDT</div>
          <div><strong>Total to pay:</strong> ${moneyUSDT(j.totalCost || 0)} USDT</div>
        </div>
      </div>
    `;
}

// Allocation
function renderAllocation(card, j) {
    const legs = Array.isArray(j.legs) ? j.legs : [];

    // best/worst по цене
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

    card.classList.remove('hidden');
    card.innerHTML = `
      <h2>Allocation</h2>
      <table>
        <thead>
          <tr>
            <th>Exchange</th>
            <th class="num">Amount (${j.base || '-'})</th>
            <th class="num">Price (USDT/${j.base || '-'})</th>
            <th class="num">Fee (USDT)</th>
          </tr>
        </thead>
        <tbody>${rows}</tbody>
        <tfoot>
          <tr>
            <th>Total</th>
            <th class="num">${qtyBASE(sumAmount(legs))}</th>
            <th></th>
            <th class="num">${moneyUSDT(sumFees(legs))}</th>
          </tr>
        </tfoot>
      </table>
    `;
}

document.addEventListener('DOMContentLoaded', () => {
    checkHealth();
    loadSymbols();

    const form = $('plan-form');
    const res  = $('result'); // Summary card
    const legs = $('legs');   // Allocation card
    const btn  = $('calc-btn');

    form?.addEventListener('submit', async (e) => {
        e.preventDefault();
        if (!btn) return;
        btn.disabled = true;

        const base = $('base')?.value?.trim().toUpperCase() || 'BTC';
        const quote = $('quote')?.value?.trim().toUpperCase() || 'USDT';
        const scenario = $('scenario')?.value || 'best_single';
        const amount = Number($('amount')?.value || 0);
        const depth = Number($('depth')?.value || 100);

        res.classList.remove('hidden');
        res.innerHTML = `<div class="muted">Calculating…</div>`;
        legs.classList.add('hidden');
        legs.innerHTML = '';

        try {
            const r = await fetch('/api/plan', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ base, quote, amount, depth, scenario })
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
