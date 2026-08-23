# AI Proxy Gateway & Semantic Cache
<!-- TODO: Update readme with complete information -->

## Database migrations

Versioned SQL migrations are stored in `internal/db/migrations`. Apply pending
migrations before starting the API:

```bash
go run ./cmd/migrate
```

Never edit a migration after it has been applied to a shared environment.
Create the next numbered migration instead.

For complete info please refer to this [doc](https://docs.google.com/document/d/1O3cy0S2w1GE0Bb_Wdn8dErxiLdOfQ3Z1AfMc4R5a3BE/edit?tab=t.0)
