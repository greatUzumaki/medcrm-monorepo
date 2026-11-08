import { ru } from './ru';
import { en } from './en';
import { zh } from './zh';
import { es } from './es';

export type Dict = typeof ru;
export const locales = ['ru', 'en', 'zh', 'es'] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = 'ru';

const dicts: Record<Locale, Dict> = { ru, en, zh, es };

export const htmlLang: Record<Locale, string> = { ru: 'ru', en: 'en', zh: 'zh-Hans', es: 'es' };
export const ogLocale: Record<Locale, string> = { ru: 'ru_RU', en: 'en_US', zh: 'zh_CN', es: 'es_ES' };
export const localeNames: Record<Locale, string> = { ru: 'Русский', en: 'English', zh: '中文', es: 'Español' };

export const t = (l: Locale): Dict => dicts[l];

/** Path of `page` ('' = home, 'privacy/') in locale `l`. ru lives at the root. */
export const localePath = (l: Locale, page = ''): string => (l === defaultLocale ? `/${page}` : `/${l}/${page}`);
