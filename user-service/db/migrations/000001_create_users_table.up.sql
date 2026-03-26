CREATE TABLE users (
                       user_id    UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
                       first_name VARCHAR(100) NOT NULL,
                       last_name  VARCHAR(100) NOT NULL,
                       email      VARCHAR(255) NOT NULL,
                       phone      VARCHAR(20)  NULL,
                       age        INTEGER      NULL CHECK (age >= 0 AND age <= 150),
                       status     VARCHAR(20)  NOT NULL DEFAULT 'ACTIVE'
                           CHECK (status IN ('ACTIVE','INACTIVE','SUSPENDED')),
                       deleted_at TIMESTAMPTZ  NULL,
                       created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
                       updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status);