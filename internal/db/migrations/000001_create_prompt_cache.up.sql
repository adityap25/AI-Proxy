CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE prompt_cache (
    id SERIAL PRIMARY KEY,
    prompt TEXT NOT NULL,
    response TEXT NOT NULL,
    embedding VECTOR(768) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX prompt_cache_embedding_idx
    ON prompt_cache
    USING hnsw (embedding vector_cosine_ops);
