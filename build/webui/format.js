import { currentLang } from './i18n.js';

export const moneyUSDT = (n) => Number(n).toLocaleString(
    currentLang === 'ru' ? 'ru-RU' : 'en-US',
    { minimumFractionDigits: 1, maximumFractionDigits: 5 }
);
export const qtyBASE = (n) => Number(n).toLocaleString(
    currentLang === 'ru' ? 'ru-RU' : 'en-US',
    { minimumFractionDigits: 1, maximumFractionDigits: 5 }
);
export const priceUSDT = (n) => Number(n).toLocaleString(
    currentLang === 'ru' ? 'ru-RU' : 'en-US',
    { minimumFractionDigits: 1, maximumFractionDigits: 5 }
);
export const qtyBASE5 = (n) => Number(n).toLocaleString(
    currentLang === 'ru' ? 'ru-RU' : 'en-US',
    { minimumFractionDigits: 5, maximumFractionDigits: 5 }
);
export const qtyQUOTE5 = (n) => Number(n).toLocaleString(
    currentLang === 'ru' ? 'ru-RU' : 'en-US',
    { minimumFractionDigits: 5, maximumFractionDigits: 5 }
);
export const priceUSDT5 = (n) => Number(n).toLocaleString(
    currentLang === 'ru' ? 'ru-RU' : 'en-US',
    { minimumFractionDigits: 5, maximumFractionDigits: 5 }
);

export function qtyCOINTerse(n) { return qtyBASE(n); }

export const formatThousandsDots = (digits) =>
    digits.replace(/\D/g, '').replace(/\B(?=(\d{3})+(?!\d))/g, '.');

export const parseThousandsDots = (val) => {
    const digits = String(val || '').replace(/\./g, '').replace(/\s/g, '');
    const n = parseInt(digits, 10);
    return Number.isFinite(n) ? n : 0;
};
