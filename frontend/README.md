# CNSDP frontend — Phase 1.5, Milestone 1

The Signal Path — Alert Investigation flagship experience, plus the alert
inventory list (`/alerts`) that is its entry point. Implements
`docs/frontend/product-experience-brief.md`. Talks to the real Go backend
only — through this dev server's own same-origin `/api` proxy
(`vite.config.ts`), never with a credential in browser code. There is no
fixture-only runtime mode: `npm run dev` requires a real backend already
running and reachable.

## Run it

A running backend is a prerequisite — see the repository root's
`docs/reference-environment.md` (Quick start) for the exact commands to
bring up PostgreSQL + the Go API via Docker Compose and load development
alerts. Once that backend is up:

```sh
cp .env.example .env   # set API_PROXY_TOKEN to match the backend's API_TOKEN
npm install
npm run dev
```

Open the printed local URL. `/` redirects to `/alerts`, the alert
inventory — real, backend-composed alerts render there once development
data has been loaded (`../scripts/dev-seed-alerts.ps1` from the
repository root). Selecting a row opens `/alerts/:alertId`, the full Dark
Evidence Map investigation screen.

Component tests and e2e specs still use real, versioned fixture data
(`src/fixtures/alert-investigation/v1.ts`) — but only as test-time `fetch`
mocking, never as an alternate runtime data source. See
`src/features/alert-investigation/lib/alertSource.ts` and
`e2e/support/` for how each layer mocks the API; `e2e/real-backend.spec.ts`
is the one spec that intentionally installs no mock and requires the real
backend instead.

## Checks

```sh
npm run typecheck
npm run lint
npm run format
npm test
npm run build
```

## Visual/interaction verification

```sh
npm run build
npm run e2e
```

Playwright runs against the production preview build
(`vite preview`), not the dev server — see `playwright.config.ts`.
Screenshots and reports are written to `review-artifacts/` and
`playwright-report/`, both gitignored; they are not committed.
`e2e/real-backend.spec.ts` additionally requires the real backend
running with development alerts loaded (see "Run it" above).
