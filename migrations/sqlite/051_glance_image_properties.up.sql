-- Migration 051: Add properties TEXT column to images table
-- Enables storing backup metadata and custom image properties

ALTER TABLE images ADD COLUMN properties TEXT DEFAULT '{}';
