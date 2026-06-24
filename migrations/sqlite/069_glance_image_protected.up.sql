-- Migration 069: Add protected column to images table
ALTER TABLE images ADD COLUMN protected INTEGER NOT NULL DEFAULT 0;
