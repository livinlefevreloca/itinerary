-- +migrate Up notransaction
CREATE INDEX idx_users_email ON users(email);
