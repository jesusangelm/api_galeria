CREATE TABLE IF NOT EXISTS items (
  id bigserial PRIMARY KEY,
  name text NOT NULL,
  description text NOT NULL,
  created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
  category_id bigserial NOT NULL REFERENCES categories ON DELETE CASCADE,
  version integer NOT NULL DEFAULT 1
);

