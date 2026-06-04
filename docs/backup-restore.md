# Backup and Restore

## Export Process
- Open `/admin/backups`.
- Use `Create backup` or `GET /admin/backups/export`.
- The backup service checkpoints SQLite, creates `hub-backup-YYYY-MM-DD-HH-MM-SS.zip`, and stores it in the configured backup directory.
- The archive includes `metadata.json` and `hub.db`.

## Import Process
- Open `/admin/backups` and choose `Validate import`.
- Upload a `.zip` backup archive from the form.
- The archive is validated before staging.
- A safety backup is created first with the current database contents.
- The staged archive is written as `hub-import-YYYY-MM-DD-HH-MM-SS.zip`.

## Backup File Format
- Zip archive
- `metadata.json`
- `hub.db`
- Additional files are ignored during validation.
- Nested or traversal paths are rejected.

## Metadata Format
- `app`: must be `hub`
- `version`: backup format version
- `database`: must be `sqlite`
- `schema_version`: current schema version number
- `created_at`: RFC3339 timestamp

Example:
```json
{
  "app": "hub",
  "version": "0.1.0",
  "database": "sqlite",
  "schema_version": 1,
  "created_at": "2026-06-04T00:00:00Z"
}
```

## Safety Backup Behavior
- `PrepareImport` always creates a safety backup before staging the uploaded archive.
- If staging fails, the safety backup is removed only when it was created by that import attempt.

## Restore Warnings
- Restore is destructive once wiring is finished.
- Verify the archive source before uploading.
- Validation rejects archives that are not for `hub`, are not SQLite, or contain invalid paths.

## CLI Restore
- `hub backup import` is not implemented yet.
- Current CLI support covers `serve`, `migrate`, `create-admin`, and `healthcheck`.

## Docker Volume Notes
- In Docker Compose, backups live in the `hub-backups` volume mounted at `/app/backups`.
- The database lives in the `hub-data` volume mounted at `/app/data`.
- Export and import both operate on these configured paths.
