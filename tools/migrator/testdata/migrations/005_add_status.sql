-- +migrate Up
-- +migrate Depends: 001 002
ALTER TABLE users ADD COLUMN status TEXT DEFAULT 'active';
