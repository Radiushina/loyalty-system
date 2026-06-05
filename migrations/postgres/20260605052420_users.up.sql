CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users(
       id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
       login TEXT NOT NULL UNIQUE,
       password TEXT NOT NULL
);