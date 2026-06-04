# Backup and Restore

## Export Process
- Open `/admin/backups`.
- Use `Create backup`.
- The archive includes `metadata.json` and `hub.db`.

## Import Process
- Upload a `.zip` backup from the backups page.
- The archive is validated before staging.
- A safety backup is created first.

## Backup File Format
- Zip archive
- `metadata.json`
- `hub.db`

## Metadata Format
- `app`
- `version`
- `database`
- `schema_version`
- `created_at`

## Safety Backup Behavior
- An existing database snapshot is created before import staging.

## Restore Warnings
- Import is destructive once the restore path is completed.
- Always verify the archive source before uploading.

## CLI Restore
- `hub backup import` is not implemented yet.

## Docker Volume Notes
- Backups are stored in the configured backup volume or directory.
