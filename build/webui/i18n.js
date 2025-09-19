export const dict = {
    en: {
        buy:'Exchanged to', pay:'Give', spend:'Amount to give', calculate:'Calculate',
        summary:'The script', allocation:'Allocation', serverOK:'Server is working',
        serverFail:'Server is not responding', pair:'Pair', receive:'Exchanged to',
        unspent:'Unspent (order book depth limit)', currentTime:'Current time',
        spendLabel:'Exchanged', avgPrice:'Average price', totalToPay:'Total to pay',
        exchange:'Give', amountCol:'Amount to give', priceCol:'Exchanged to',
        total:'Total', per:'per 1', best_single:'All funds to one exchange',
        equal_split:'Even distribution', optimal:'AI distribution', usdt:'USDT',
        calculating:'Calculating…', resultsFor:'Results for pair', diffCol:'Difference',
    },
    ru: {
        buy:'Обмениваем на', pay:'Отдаём', spend:'Сколько отдаём', calculate:'Рассчитать',
        summary:'Сценарий', allocation:'Распределение', serverOK:'Сервер работает',
        serverFail:'Сервер не отвечает', pair:'Пара', receive:'Обмениваем на',
        unspent:'Не израсходовано (ограничение глубины)', currentTime:'Текущее время',
        spendLabel:'Обменяли', avgPrice:'Средняя цена', totalToPay:'Итого к оплате',
        exchange:'Отдаём', amountCol:'Сколько отдаём', priceCol:'Обмениваем на',
        total:'Итого', per:'за 1', best_single:'Все деньги в одну биржу',
        equal_split:'Равномерное распределение средств', optimal:'AI распределение',
        usdt:'USDT', calculating:'Расчёт…', resultsFor:'Результаты для пары',
        diffCol:'Разница',
    }
};

export let currentLang = localStorage.getItem('lang') || 'ru';

export function setLang(lang) {
    currentLang = lang;
    localStorage.setItem('lang', lang);
    document.querySelectorAll('#lang-toggle .lang-btn')?.forEach(btn => {
        btn.classList.toggle('active', btn.id === `btn-${lang}`);
    });
    const t = dict[lang];
    const $ = (id) => document.getElementById(id);
    const elBuy = $('lbl-buy');     if (elBuy) elBuy.textContent = t.buy;
    const elPay = $('lbl-pay');     if (elPay) elPay.textContent = t.pay;
    const elSpend = $('lbl-spend'); if (elSpend) elSpend.textContent = t.spend;
    const elCalc = $('calc-btn');   if (elCalc) elCalc.textContent = t.calculate;
    const health = $('health');
    if (health && !health.classList.contains('bad')) health.textContent = t.serverOK;
    const fs = $('footer-status');
    if (fs && !health?.classList.contains('bad')) fs.textContent = t.serverOK;
}

export function scenarioTitle(s) {
    const t = dict[currentLang];
    if (s === 'best_single') return t.best_single;
    if (s === 'equal_split') return t.equal_split;
    if (s === 'optimal') return t.optimal;
    return s || '';
}
