package app

import (
	"io"
	"html"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/services"
	"github.com/AmooVPM/hub/internal/security"
)

func (r *Runner) getAdminBackups(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	if !security.HasRole(admin.Role, security.RoleOwner, security.RoleAdmin) {
		return c.Status(fiber.StatusForbidden).Type("html").SendString(renderAdminUsersPageMessage("forbidden", r.cfg.AppName))
	}
	items, err := r.backups.List(r.cfg.BackupDir)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminBackupsPage(items, admin.Role, r.cfg.AppName, r.cfg.BackupDir))
}

func (r *Runner) getAdminBackupsExport(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	if !security.HasRole(admin.Role, security.RoleOwner, security.RoleAdmin) {
		return c.Status(fiber.StatusForbidden).Type("html").SendString(renderAdminUsersPageMessage("forbidden", r.cfg.AppName))
	}
	ctx := c.UserContext()
	if _, err := r.db.ExecContext(ctx, `PRAGMA wal_checkpoint(FULL);`); err != nil {
		return err
	}
	rec, err := r.backups.Create(ctx, r.cfg.DatabasePath, r.cfg.BackupDir, r.cfg.AppName)
	if err != nil {
		return err
	}
	if deleted, cleanupErr := r.backups.CleanupOldBackups(r.cfg.BackupDir, r.cfg.BackupRetentionCount, r.cfg.BackupRetentionDays, rec.Name); cleanupErr != nil {
		if r.logger != nil {
			r.logger.Warn("backup retention cleanup failed", "error", cleanupErr)
		}
	} else if len(deleted) > 0 && r.logger != nil {
		r.logger.Info("backup retention cleanup completed", "deleted", len(deleted))
	}
	path, err := r.backups.Path(r.cfg.BackupDir, rec.Name)
	if err != nil {
		return err
	}
	_ = r.logAudit(ctx, "admin", adminActorID(admin), "backup_export", "backup", nil, map[string]any{"filename": rec.Name})
	return c.Download(path, rec.Name)
}

func (r *Runner) getAdminBackupsDownload(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	if !security.HasRole(admin.Role, security.RoleOwner, security.RoleAdmin) {
		return c.Status(fiber.StatusForbidden).Type("html").SendString(renderAdminUsersPageMessage("forbidden", r.cfg.AppName))
	}
	path, err := r.backups.Path(r.cfg.BackupDir, c.Params("filename"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return c.Download(path, filepath.Base(path))
}

func (r *Runner) postAdminBackupsDelete(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	if !security.HasRole(admin.Role, security.RoleOwner) {
		return c.Status(fiber.StatusForbidden).Type("html").SendString(renderAdminUsersPageMessage("forbidden", r.cfg.AppName))
	}
	if err := r.backups.Delete(r.cfg.BackupDir, c.Params("filename")); err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "backup_delete", "backup", nil, map[string]any{"filename": c.Params("filename")})
	return c.Redirect("/admin/backups", fiber.StatusFound)
}

func (r *Runner) postAdminBackupsImport(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	if !security.HasRole(admin.Role, security.RoleOwner) {
		return c.Status(fiber.StatusForbidden).Type("html").SendString(renderAdminUsersPageMessage("forbidden", r.cfg.AppName))
	}
	file, err := c.FormFile("backup")
	if err != nil {
		items, listErr := r.backups.List(r.cfg.BackupDir)
		if listErr != nil {
			return listErr
		}
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminBackupsPageWithAlert(items, admin.Role, r.cfg.AppName, r.cfg.BackupDir, "Choose a backup file to upload.", "warning"))
	}
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".zip") {
		items, listErr := r.backups.List(r.cfg.BackupDir)
		if listErr != nil {
			return listErr
		}
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminBackupsPageWithAlert(items, admin.Role, r.cfg.AppName, r.cfg.BackupDir, "Backup files must use the .zip extension.", "warning"))
	}
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	data, err := io.ReadAll(src)
	if err != nil {
		return err
	}
	result, err := r.backups.PrepareImport(c.UserContext(), data, r.cfg.DatabasePath, r.cfg.BackupDir, r.cfg.AppName)
	if err != nil {
		_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "backup_import_failed", "backup", nil, map[string]any{"filename": file.Filename, "error": err.Error()})
		items, listErr := r.backups.List(r.cfg.BackupDir)
		if listErr != nil {
			return listErr
		}
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminBackupsPageWithAlert(items, admin.Role, r.cfg.AppName, r.cfg.BackupDir, err.Error(), "danger"))
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "backup_import_prepare", "backup", nil, map[string]any{"filename": file.Filename, "app": result.Metadata.App, "schema_version": result.Metadata.SchemaVersion, "safety_backup": result.SafetyBackup.Name, "staged_file": result.StagedName})
	items, err := r.backups.List(r.cfg.BackupDir)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminBackupsPageWithAlert(items, admin.Role, r.cfg.AppName, r.cfg.BackupDir, "Backup validated and safety backup created. Restore wiring is still pending.", "success"))
}

func renderAdminBackupsPage(items []services.BackupRecord, adminRole, appName, backupDir string) string {
	return renderAdminBackupsPageWithAlert(items, adminRole, appName, backupDir, "", "")
}

func renderAdminBackupsPageWithAlert(items []services.BackupRecord, adminRole, appName, backupDir, message, alertClass string) string {
	var rows strings.Builder
	for _, item := range items {
		actions := `<a class="btn btn-outline-secondary btn-sm" href="/admin/backups/download/` + html.EscapeString(item.Name) + `">Download</a>`
		if security.HasRole(adminRole, security.RoleOwner) {
			actions += ` <form method="post" action="/admin/backups/delete/` + html.EscapeString(item.Name) + `" class="d-inline" onsubmit="return confirm('Delete this backup?')"><button class="btn btn-outline-danger btn-sm" type="submit">Delete</button></form>`
		}
		rows.WriteString(`<tr><td>` + html.EscapeString(item.Name) + `</td><td>` + html.EscapeString(formatBytes(item.Size)) + `</td><td>` + html.EscapeString(item.ModifiedAt.Format(time.RFC3339)) + `</td><td class="text-nowrap">` + actions + `</td></tr>`)
	}
	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="4" class="text-body-secondary">No backups found.</td></tr>`)
	}
	var alert string
	if strings.TrimSpace(message) != "" {
		if strings.TrimSpace(alertClass) == "" {
			alertClass = "info"
		}
		alert = `<div class="alert alert-` + html.EscapeString(alertClass) + `">` + html.EscapeString(message) + `</div>`
	}
	importForm := `<div class="card shadow-sm"><div class="card-body"><h2 class="h5">Import</h2><p class="text-body-secondary">Importing a backup will replace the current database. A safety backup will be created automatically before import.</p><form method="post" action="/admin/backups/import" enctype="multipart/form-data" class="vstack gap-3"><div><label class="form-label" for="backup">Backup archive</label><input class="form-control" id="backup" name="backup" type="file" accept=".zip" required></div><div class="d-flex gap-2"><button class="btn btn-warning" type="submit">Validate import</button></div></form></div></div>`
	if !security.HasRole(adminRole, security.RoleOwner) {
		importForm = `<div class="card shadow-sm"><div class="card-body"><h2 class="h5">Import</h2><p class="text-body-secondary mb-0">Import is restricted to owners.</p></div></div>`
	}
	exportButton := `<a class="btn btn-primary btn-sm" href="/admin/backups/export">Create backup</a>`
	body := `<div class="container py-4 py-lg-5">` + alert + `<div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">Backups</h1><p class="text-body-secondary mb-0">Backup directory: ` + html.EscapeString(backupDir) + `</p></div><div class="d-flex gap-2">` + exportButton + `<a class="btn btn-outline-secondary btn-sm" href="/admin">Back</a></div></div><div class="row g-3 mb-3"><div class="col-12">` + importForm + `</div></div><div class="card shadow-sm"><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Filename</th><th>Size</th><th>Modified</th><th>Actions</th></tr></thead><tbody>` + rows.String() + `</tbody></table></div></div></div>`
	return renderAdminShell(appName, adminRole, "backups", body)
}
