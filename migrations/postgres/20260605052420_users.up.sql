CREATE TABLE IF NOT EXISTS users(
       id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
       login VARCHAR(200) NOT NULL UNIQUE,
       password TEXT NOT NULL
);