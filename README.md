# AI Proxy Gateway & Semantic Cache

## Semantic-cache contract

This is currently a single-system deployment: every request is in the same
authorization domain, so cache entries are not tenant-scoped. Before making
the service multi-tenant, include the authenticated organization or user scope
in the cache scope and only search within it.

A response may be reused only when all of the following match:

- generation model;
- embedding model;
- application system-prompt version; and
- an unexpired entry has cosine similarity at or above
  `CACHE_SIMILARITY_THRESHOLD`.

`CACHE_SIMILARITY_THRESHOLD` is a similarity value from 0 through 1. Pgvector's
cosine operator returns *distance*, therefore a configured similarity of 0.90
will be queried as a maximum cosine distance of 0.10. Change
`SYSTEM_PROMPT_VERSION` whenever the system prompt or answer policy changes;
this prevents answers produced under old instructions being returned under new
ones.

The cache will fail open: if its database lookup or embedding call is
unavailable, the proxy will generate an answer normally rather than returning a
cache infrastructure error. Only safe, reusable prompt/response pairs should
be cached; do not use the semantic cache for live data, user-specific data, or
side-effectful operations.

## Database migrations

Versioned SQL migrations are stored in `internal/db/migrations`. Apply pending
migrations before starting the API:

```bash
go run ./cmd/migrate
```

Never edit a migration after it has been applied to a shared environment.
Create the next numbered migration instead.

For complete info please refer to this [doc](https://docs.google.com/document/d/1O3cy0S2w1GE0Bb_Wdn8dErxiLdOfQ3Z1AfMc4R5a3BE/edit?tab=t.0)
