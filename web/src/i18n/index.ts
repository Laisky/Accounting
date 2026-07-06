import i18n from 'i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import { initReactI18next } from 'react-i18next';

import en from './locales/en';

export type SupportedLanguage = 'en' | 'zh' | 'fr' | 'es' | 'ja';
type LocaleModule = { default: Record<string, unknown> };

export const supportedLanguages: SupportedLanguage[] = ['en', 'zh', 'fr', 'es', 'ja'];

const localeLoaders: Record<Exclude<SupportedLanguage, 'en'>, () => Promise<LocaleModule>> = {
  zh: () => import('./locales/zh'),
  fr: () => import('./locales/fr'),
  es: () => import('./locales/es'),
  ja: () => import('./locales/ja'),
};

const loadedLanguages = new Set<SupportedLanguage>(['en']);

// normalizeLanguage receives a raw language tag and returns the closest supported language, defaulting to English.
const normalizeLanguage = (lng?: string): SupportedLanguage => {
  const baseLanguage = lng?.split('-')[0] as SupportedLanguage | undefined;
  return baseLanguage && supportedLanguages.includes(baseLanguage) ? baseLanguage : 'en';
};

// loadLanguageResources receives a language tag, lazily loads its translation bundle once, and returns the resolved language.
const loadLanguageResources = async (lng?: string): Promise<SupportedLanguage> => {
  const language = normalizeLanguage(lng);
  if (loadedLanguages.has(language)) {
    return language;
  }
  if (language === 'en') {
    return language;
  }

  const locale = await localeLoaders[language]();
  i18n.addResourceBundle(language, 'translation', locale.default, true, true);
  loadedLanguages.add(language);
  return language;
};

// changeAppLanguage receives a language tag, ensures its bundle is loaded, and switches the active language.
export const changeAppLanguage = async (lng: string): Promise<void> => {
  const language = await loadLanguageResources(lng);
  await i18n.changeLanguage(language);
};

const resources = {
  en: { translation: en },
};

i18n
  // Detect user language
  .use(LanguageDetector)
  // Pass the i18n instance to react-i18next
  .use(initReactI18next)
  // Init i18next
  .init({
    resources,
    fallbackLng: 'en', // Default fallback
    supportedLngs: supportedLanguages,
    partialBundledLanguages: true,
    debug: import.meta.env.MODE === 'development',

    interpolation: {
      escapeValue: false, // React already safes from xss
    },

    react: {
      // Bundled English keeps first render synchronous, so Suspense is unnecessary.
      useSuspense: false,
    },

    detection: {
      // Order and from where user language should be detected
      order: ['localStorage', 'navigator'],
      // Keys or params to lookup language from
      lookupLocalStorage: 'i18nextLng',
      // Cache user language on
      caches: ['localStorage'],
    },
  });

// syncHtmlLang receives a language tag and mirrors it onto the <html lang> attribute.
const syncHtmlLang = (lng: string) => {
  document.documentElement.lang = normalizeLanguage(lng);
};
syncHtmlLang(i18n.language);
i18n.on('languageChanged', (lng) => {
  syncHtmlLang(lng);
  const language = normalizeLanguage(lng);
  const alreadyLoaded = loadedLanguages.has(language);
  void loadLanguageResources(lng).then((resolved) => {
    if (!alreadyLoaded && normalizeLanguage(i18n.language) === resolved) {
      void i18n.changeLanguage(resolved);
    }
  });
});

void loadLanguageResources(i18n.language).then((language) => {
  if (language !== 'en') {
    void i18n.changeLanguage(language);
  }
});
