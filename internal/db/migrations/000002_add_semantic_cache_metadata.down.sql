ALTER TABLE prompt_cache
    DROP COLUMN IF EXISTS hit_count,
    DROP COLUMN IF EXISTS last_accessed_at,
    DROP COLUMN IF EXISTS expires_at,
    DROP COLUMN IF EXISTS cache_scope;
