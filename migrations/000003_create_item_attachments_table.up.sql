CREATE TABLE IF NOT EXISTS item_attachments (
  id bigserial PRIMARY KEY,
  key text NOT NULL,
  filename text NOT NULL,
  content_type text NOT NULL,
  byte_size bigint NOT NULL,
  created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
  item_id bigserial NOT NULL REFERENCES items ON DELETE CASCADE
)
