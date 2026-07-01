-- 启用 pg_trgm 扩展，支持 users.nickname 的 GIN trigram 索引。
CREATE EXTENSION IF NOT EXISTS pg_trgm;
