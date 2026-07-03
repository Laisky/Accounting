import { ArrowRight, BookOpenCheck, FileCheck2, Landmark, Route, ShieldCheck, UsersRound } from 'lucide-react';
import { type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { LanguageSelector } from '../../components/LanguageSelector';
import { type RuntimeConfig } from '../../lib/api/runtimeConfig';
import './landing.css';

type LandingPageProps = {
  runtimeConfig: RuntimeConfig;
};

const proofKeys = ['ownership', 'audit', 'currency'] as const;
const workflowKeys = ['personal', 'shared', 'travel'] as const;
const trustKeys = ['auth', 'imports', 'portable'] as const;

// LandingPage renders the public product introduction while keeping authentication behind explicit sign-in routes.
export function LandingPage({ runtimeConfig }: LandingPageProps) {
  const { t } = useTranslation();
  const signInHref = signInPath(runtimeConfig);
  const registerHref = runtimeConfig.auth.emailRegisterEnabled ? '/register' : signInHref;

  return (
    <main className="landingShell">
      <header className="landingHeader">
        <a className="landingBrand" href="#top" aria-label={t('landing.header.brandAria')}>
          <BookOpenCheck size={24} aria-hidden="true" />
          <span>{t('landing.header.brand')}</span>
        </a>
        <nav className="landingNav" aria-label={t('landing.header.navLabel')}>
          <a href="#workflows">{t('landing.header.workflows')}</a>
          <a href="#trust">{t('landing.header.trust')}</a>
          <a href="#model">{t('landing.header.model')}</a>
        </nav>
        <div className="landingActions">
          <LanguageSelector className="landingLanguage" />
          <AuthLink href={signInHref} className="landingSignIn">
            {t('landing.header.signIn')}
          </AuthLink>
        </div>
      </header>

      <section id="top" className="landingHero" aria-labelledby="landing-title">
        <div className="landingHeroCopy">
          <p className="eyebrow">{t('landing.hero.eyebrow')}</p>
          <h1 id="landing-title">{t('landing.hero.title')}</h1>
          <p className="landingLead">{t('landing.hero.lead')}</p>
          <div className="landingHeroActions">
            <AuthLink href={registerHref} className="landingPrimaryAction">
              <span>{runtimeConfig.auth.emailRegisterEnabled ? t('landing.hero.primaryRegister') : t('landing.hero.primarySignIn')}</span>
              <ArrowRight size={18} aria-hidden="true" />
            </AuthLink>
            <a className="landingSecondaryAction" href="#model">
              {t('landing.hero.secondary')}
            </a>
          </div>
        </div>

        <div className="landingProductShot" aria-label={t('landing.productShot.label')}>
          <div className="ledgerWindow">
            <div className="ledgerWindowBar">
              <span>{t('landing.productShot.windowTitle')}</span>
              <strong>{t('landing.productShot.status')}</strong>
            </div>
            <div className="ledgerWorkspacePreview">
              <div className="ledgerBalancePreview">
                <span>{t('landing.productShot.balanceLabel')}</span>
                <strong>{t('landing.productShot.balanceValue')}</strong>
              </div>
              <div className="ledgerEntryPreview">
                <span>{t('landing.productShot.entryOneMeta')}</span>
                <strong>{t('landing.productShot.entryOneTitle')}</strong>
                <small>{t('landing.productShot.entryOneNote')}</small>
              </div>
              <div className="ledgerEntryPreview ledgerEntryIncome">
                <span>{t('landing.productShot.entryTwoMeta')}</span>
                <strong>{t('landing.productShot.entryTwoTitle')}</strong>
                <small>{t('landing.productShot.entryTwoNote')}</small>
              </div>
              <div className="ledgerAuditPreview">
                <FileCheck2 size={18} aria-hidden="true" />
                <span>{t('landing.productShot.audit')}</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="landingProof" aria-label={t('landing.proof.label')}>
        {proofKeys.map((key) => (
          <article key={key}>
            <span>{t(`landing.proof.${key}.kicker`)}</span>
            <strong>{t(`landing.proof.${key}.title`)}</strong>
            <p>{t(`landing.proof.${key}.body`)}</p>
          </article>
        ))}
      </section>

      <section id="workflows" className="landingSection landingWorkflow" aria-labelledby="landing-workflows-title">
        <div className="landingSectionIntro">
          <p className="eyebrow">{t('landing.workflow.eyebrow')}</p>
          <h2 id="landing-workflows-title">{t('landing.workflow.title')}</h2>
        </div>
        <div className="landingWorkflowGrid">
          {workflowKeys.map((key) => (
            <article key={key} className="landingWorkflowItem">
              <WorkflowIcon name={key} />
              <h3>{t(`landing.workflow.${key}.title`)}</h3>
              <p>{t(`landing.workflow.${key}.body`)}</p>
            </article>
          ))}
        </div>
      </section>

      <section id="trust" className="landingSection landingTrust" aria-labelledby="landing-trust-title">
        <div className="landingSectionIntro">
          <p className="eyebrow">{t('landing.trust.eyebrow')}</p>
          <h2 id="landing-trust-title">{t('landing.trust.title')}</h2>
        </div>
        <div className="landingTrustGrid">
          {trustKeys.map((key) => (
            <article key={key}>
              <ShieldCheck size={22} aria-hidden="true" />
              <h3>{t(`landing.trust.${key}.title`)}</h3>
              <p>{t(`landing.trust.${key}.body`)}</p>
            </article>
          ))}
        </div>
      </section>

      <section id="model" className="landingModel" aria-labelledby="landing-model-title">
        <div>
          <p className="eyebrow">{t('landing.model.eyebrow')}</p>
          <h2 id="landing-model-title">{t('landing.model.title')}</h2>
        </div>
        <p>{t('landing.model.body')}</p>
      </section>
    </main>
  );
}

function AuthLink({ href, className, children }: { href: string; className: string; children: ReactNode }) {
  if (href.startsWith('/') && !href.startsWith('/api/')) {
    return (
      <Link className={className} to={href}>
        {children}
      </Link>
    );
  }

  return (
    <a className={className} href={href}>
      {children}
    </a>
  );
}

function WorkflowIcon({ name }: { name: (typeof workflowKeys)[number] }) {
  if (name === 'shared') {
    return <UsersRound size={24} aria-hidden="true" />;
  }
  if (name === 'travel') {
    return <Route size={24} aria-hidden="true" />;
  }

  return <Landmark size={24} aria-hidden="true" />;
}

function signInPath(runtimeConfig: RuntimeConfig): string {
  if (!runtimeConfig.auth.emailLoginEnabled && runtimeConfig.features.externalSsoEnabled && runtimeConfig.sso.startPath) {
    return runtimeConfig.sso.startPath;
  }

  return '/login';
}
