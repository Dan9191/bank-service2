CREATE DATABASE bank;

\c bank;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
                       id SERIAL PRIMARY KEY,
                       email VARCHAR(255) UNIQUE NOT NULL,
                       username VARCHAR(50) UNIQUE NOT NULL,
                       password_hash VARCHAR(255) NOT NULL,
                       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE accounts (
                          id SERIAL PRIMARY KEY,
                          user_id INT REFERENCES users(id),
                          balance DECIMAL(15,2) DEFAULT 0.00,
                          currency CHAR(3) DEFAULT 'RUB',
                          created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);