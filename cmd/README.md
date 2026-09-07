# cmd

## app

This is the example application entry point for Sphere. It runs the dashboard
HTTP server (dash + file routes) and the Telegram bot.

## tools

- `config`: Generate configuration example files.
- `gen/ent`: Generate `ent` code for the database schema.
- `gen/entmap`: Generate `ent` → `entpb` mapping helpers.
- `gen/entcrud`: Generate entity create/update binding code.
