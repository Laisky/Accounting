# Public Landing Page Research

The Accounting homepage should explain the product before asking for authentication. Current UI research favors a public first page that makes scope, audience, trust posture, and the next action clear, while keeping sign-in available as a conventional header action.

## Findings

- NN/g's homepage guidance emphasizes that homepages must quickly communicate what the product does, who it is for, and what users can accomplish. Accounting's homepage should describe shared bookkeeping scenarios before sending visitors to authentication. Source: https://www.nngroup.com/articles/homepage-design-principles/
- NN/g's CTA guidance warns against generic "Get Started" actions when users do not yet understand the service. Accounting should use specific actions such as creating a ledger, viewing sign-in options, or seeing the data model. Source: https://www.nngroup.com/articles/get-started/
- Baymard's homepage research shows users infer a product's scope from visible homepage content. Accounting should expose representative scenarios early: personal finance, shared books, trips, reimbursements, projects, and small teams. Source: https://baymard.com/blog/inferring-product-catalog-from-homepage/
- Unbounce's SaaS benchmark highlights the importance of mobile traffic for SaaS landing pages. The landing header, sign-in action, and primary action must remain reachable without hover-only interaction. Source: https://unbounce.com/conversion-benchmark-report/saas-conversion-rate/
- CXL's B2B landing-page guidance prioritizes message clarity, proof, trust signals, CTA mechanics, and usability. For Accounting, trust signals should be concrete: audit events, user-owned data, review-before-write imports, and runtime-discovered authentication methods. Source: https://cxl.com/blog/landing-page-infrastructure/
- Webflow's SaaS design examples and NN/g homepage guidance both favor product-revealing visuals over abstract imagery. Accounting should show a ledger state, transaction review, or audit trail rather than decorative finance imagery. Source: https://webflow.com/blog/saas-website-design-examples
- Figma's 2026 trend report highlights bold hierarchy, motion, and accessibility-minded design. Accounting should use those ideas with restraint because financial tools need clarity and trust more than novelty. Source: https://www.figma.com/resource-library/web-design-trends/
- WCAG 2.2 applies to both the landing page and the auth entry points. Header actions need visible focus, keyboard access, consistent navigation, adequate target size, and accessible authentication support. Source: https://www.w3.org/TR/WCAG22/
- Core Web Vitals remain a product-quality constraint: landing visuals should avoid heavy embeds, layout shifts, and delayed first meaningful content. Source: https://web.dev/articles/vitals

## Product Implications

- `/` is a public product landing page for signed-out visitors; it is not the login form.
- The header owns the returning-user `Sign in` action. It routes to `/login` unless the deployment is SSO-only, in which case it can route to the configured SSO start path.
- The hero should include concrete product copy and a product-revealing ledger preview.
- The page should include use cases, data-integrity claims, and the ownership model near the first page view.
- Landing copy must use i18n keys, and the route must preserve existing authenticated deep-link behavior.
