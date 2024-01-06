CREATE EXTENSION IF NOT EXISTS citext;
CREATE TABLE IF NOT EXISTS categories (
  id bigserial PRIMARY KEY,
  name citext UNIQUE NOT NULL,
  description text NOT NULL,
  created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
  version integer NOT NULL DEFAULT 1
);
