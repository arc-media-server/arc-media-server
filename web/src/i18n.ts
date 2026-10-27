import i18n from 'i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import { initReactI18next } from 'react-i18next';

// Every English namespace is bundled. Other languages produce separate, on-demand chunks.
const english = import.meta.glob<{ default: Record<string, unknown> }>('./locales/en/*.json', { eager: true });
const translations = import.meta.glob<{ default: Record<string, unknown> }>('./locales/*/*.json');
const resources = Object.fromEntries(Object.entries(english).map(([path, module]) => [path.split('/').at(-1)!.replace('.json', ''), module.default]));
const supportedLngs = [...new Set(Object.keys(translations).map(path => path.split('/')[2]))];

i18n.use({
  type: 'backend',
  read(language: string, namespace: string, callback: (error: unknown, data: unknown) => void) {
    const load = translations[`./locales/${language}/${namespace}.json`];
    if (!load) { callback(null, {}); return; }
    void load().then(module => callback(null, module.default), error => callback(error, false));
  },
}).use(LanguageDetector).use(initReactI18next);

// Start synchronously in English, including when an available translation is still loading.
void i18n.init({
  lng: 'en',
  fallbackLng: 'en',
  supportedLngs,
  resources: { en: resources },
  partialBundledLanguages: true,
  ns: Object.keys(resources),
  defaultNS: 'common',
  initAsync: false,
  interpolation: { escapeValue: false },
  react: { useSuspense: false },
  detection: { order: ['navigator'], caches: [] },
});

const detected = i18n.services.languageDetector?.detect();
const preferred = i18n.services.languageUtils.getBestMatchFromCodes(typeof detected === 'string' ? [detected] : detected ?? []);
// Unsupported browser languages never request a nonexistent translation file.
export const languageReady = preferred && preferred !== 'en' ? i18n.changeLanguage(preferred) : Promise.resolve();
export default i18n;
