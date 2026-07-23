# CNSDP frontend — Phase 1.5, Milestone 1

The Signal Path — Alert Investigation flagship prototype. Implements
`docs/frontend/product-experience-brief.md`. Runs entirely independently of
the Go backend on real, versioned fixture data
(`src/fixtures/alert-investigation/v1.ts`) — no backend process, database, or
Docker Compose is required to run or review it.

## Run it

```sh
npm install
npm run dev
```

Open the printed local URL. The root route redirects to `/alerts/1` (the
happy-path fixture — full evidence availability, intact traceability).

Other fixture ids demonstrate the remaining required presentation states:

| URL                           | State                                                            |
| ----------------------------- | ---------------------------------------------------------------- |
| `/alerts/1`                   | Full evidence availability, traceability intact                  |
| `/alerts/2`                   | Partial artifact availability (detection definition unavailable) |
| `/alerts/3`                   | Broken traceability (`raw_event_sha256`)                         |
| `/alerts/999`                 | Alert not found                                                  |
| `/alerts/1?demo=unauthorized` | Unauthorized (401)                                               |
| `/alerts/1?demo=unavailable`  | Backend unavailable                                              |
| `/alerts/1?demo=slow`         | Extended latency, to observe the loading state                   |

The `?demo=` override exists only in this fixture harness — see
`src/features/alert-investigation/lib/alertSource.ts` for why it has no real
backend equivalent and will not survive the switch to a live API.

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

## Replacing fixtures with the real backend

`src/features/alert-investigation/lib/alertSource.ts` is the one seam a real
`fetch("/v1/alerts/" + id)` call replaces — every consumer only ever sees an
`AlertInvestigationResponse` or an `AlertFetchError`, exactly the shape a
real request against `GET /v1/alerts/{id}` would produce. No other file
needs to change (product-experience-brief.md §10.1).
