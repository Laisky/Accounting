import { Languages } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { changeAppLanguage, supportedLanguages, type SupportedLanguage } from '../i18n';
import './language-selector.css';

type LanguageOption = { code: SupportedLanguage; label: string };

const languages: LanguageOption[] = [
  { code: 'en', label: 'English' },
  { code: 'zh', label: '简体中文' },
  { code: 'fr', label: 'Français' },
  { code: 'es', label: 'Español' },
  { code: 'ja', label: '日本語' },
];

type LanguageSelectorProps = {
  className?: string;
};

// LanguageSelector receives an optional class name and returns an accessible language switcher backed by a native select.
export function LanguageSelector({ className }: LanguageSelectorProps) {
  const { i18n, t } = useTranslation();
  const current = normalizeCurrent(i18n.language);
  const label = t('mobile.a11y.selectLanguage');

  // handleChange receives a select change event and switches the active application language.
  const handleChange = (event: React.ChangeEvent<HTMLSelectElement>) => {
    void changeAppLanguage(event.target.value);
  };

  return (
    <label className={`languageSelector ${className ?? ''}`.trim()}>
      <span className="languageSelectorLabel">
        <Languages size={16} aria-hidden="true" />
        {label}
      </span>
      <select aria-label={label} value={current} onChange={handleChange}>
        {languages.map((language) => (
          <option key={language.code} value={language.code}>
            {language.label}
          </option>
        ))}
      </select>
    </label>
  );
}

// normalizeCurrent receives the active i18next language and returns the matching supported base language.
function normalizeCurrent(language: string | undefined): SupportedLanguage {
  const base = language?.split('-')[0] as SupportedLanguage | undefined;
  return base && supportedLanguages.includes(base) ? base : 'en';
}
