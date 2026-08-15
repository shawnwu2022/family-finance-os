# ezBookkeeping HTTP API fixtures

Baseline: ezBookkeeping 1.6.1, reviewed 2026-08-15.

These files contain synthetic, sanitized values shaped strictly to the official HTTP API response schemas. They do not contain production financial data.

Official schema references:

- API envelope/auth/timezone: https://ezbookkeeping.mayswind.net/httpapi/
- Accounts: https://ezbookkeeping.mayswind.net/httpapi/account_api
- Transaction categories: https://ezbookkeeping.mayswind.net/httpapi/transaction_category_api
- Transactions/pagination: https://ezbookkeeping.mayswind.net/httpapi/transaction_api

When the pinned ezBookkeeping version changes, compare these fixtures and adapter enum mappings against the upstream documentation before updating the version in production.
