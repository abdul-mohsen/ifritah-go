package handlers

// ============================================================================
// File Storage — Generic Upload-First, Reference-Later Pattern
//
// FLOW:
//   1. User picks a file in the browser (bill PDF or attachment)
//   2. Frontend immediately uploads it:  POST /api/v2/upload
//   3. Backend saves file to disk, returns a file_key (download URL)
//   4. Frontend stores the file_key in a hidden input / JS state
//   5. User fills form and clicks "Submit Purchase Bill"
//   6. Frontend sends the purchase bill payload WITH file references:
//        POST /api/v2/purchase_bill
//        {
//          "store_id": 1,
//          "supplier_id": 5,
//          "pdf_link": "/api/v2/files/abc123def456.pdf",
//          "attachments": [
//            "/api/v2/files/789ghi012jkl.pdf",
//            "/api/v2/files/mno345pqr678.jpg"
//          ],
//          "products": [...],
//          ...
//        }
//   7. Backend creates the bill and associates the file references
//
// Endpoints:
//   POST   /api/v2/upload          — Generic file upload (returns file_key + download URL)
//   GET    /api/v2/files/:key      — Download/view a file by its key
//   DELETE /api/v2/files/:key      — Delete an uploaded file
//
// Storage layout:
//   uploads/
//   └── files/
//       ├── abc123def456.pdf       ← uploaded files, keyed by random hex
//       ├── 789ghi012jkl.pdf
//       └── mno345pqr678.jpg
//
// Database: uploaded_files table tracks metadata + ownership
// Security: JWT required for upload, files served via key (no directory listing)
// ============================================================================

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ── Configuration ───────────────────────────────────────────────────────────

const (
	// FilesUploadDir is where all uploaded files are stored on disk.
	FilesUploadDir = "uploads/files"
	// MaxFileSize — 10 MB per file.
	MaxFileSize = 10 << 20
)

// Allowed file extensions for upload.
var allowedUploadExt = map[string]bool{
	".pdf":  true,
	".jpg":  true,
	".jpeg": true,
	".png":  true,
}

// MIME types by extension.
var mimeTypes = map[string]string{
	".pdf":  "application/pdf",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
}

// ── Database Schema ─────────────────────────────────────────────────────────
//
// CREATE TABLE IF NOT EXISTS uploaded_files (
//   id            INT UNSIGNED    NOT NULL AUTO_INCREMENT,
//   file_key      VARCHAR(100)    NOT NULL COMMENT 'Random hex key: abc123def456.pdf',
//   original_name VARCHAR(255)    NOT NULL,
//   file_size     BIGINT UNSIGNED NOT NULL DEFAULT 0,
//   mime_type     VARCHAR(100)    NULL,
//   uploaded_by   INT UNSIGNED    NULL,
//   created_at    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
//
//   PRIMARY KEY (id),
//   UNIQUE KEY uq_file_key (file_key),
//   INDEX idx_uploaded_by (uploaded_by),
//
//   CONSTRAINT fk_upload_user FOREIGN KEY (uploaded_by)
//     REFERENCES users(id) ON DELETE SET NULL ON UPDATE CASCADE
// ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
//
// -- Then modify purchase_bill to include file references:
// ALTER TABLE purchase_bill
//   ADD COLUMN pdf_link     VARCHAR(255) NULL COMMENT 'file_key of the mandatory bill PDF';
//
// -- Optional: a join table for multiple attachments per bill.
// -- Or just store a JSON array in purchase_bill.attachments TEXT column.
// -- Option A: JSON column (simpler)
// ALTER TABLE purchase_bill
//   ADD COLUMN attachments JSON NULL COMMENT 'JSON array of file_keys';
//
// -- Option B: Join table (normalized, better for queries)
// CREATE TABLE IF NOT EXISTS purchase_bill_attachments (
//   id                INT UNSIGNED NOT NULL AUTO_INCREMENT,
//   purchase_bill_id  INT UNSIGNED NOT NULL,
//   file_key          VARCHAR(100) NOT NULL,
//   created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
//
//   PRIMARY KEY (id),
//   INDEX idx_pba_bill (purchase_bill_id),
//   UNIQUE KEY uq_pba_file (purchase_bill_id, file_key),
//
//   CONSTRAINT fk_pba_bill FOREIGN KEY (purchase_bill_id)
//     REFERENCES purchase_bill(id) ON DELETE CASCADE,
//   CONSTRAINT fk_pba_file FOREIGN KEY (file_key)
//     REFERENCES uploaded_files(file_key) ON DELETE CASCADE
// ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
//

// ── Response Models ─────────────────────────────────────────────────────────

// UploadResponse is returned after a successful file upload.
type UploadResponse struct {
	FileKey      string `json:"file_key"`
	OriginalName string `json:"original_name"`
	FileSize     int64  `json:"file_size"`
	MimeType     string `json:"mime_type"`
	DownloadURL  string `json:"download_url"`
}

// ══════════════════════════════════════════════════════════════════════════════
// GENERIC FILE UPLOAD
// ══════════════════════════════════════════════════════════════════════════════

// UploadFile is a generic file upload endpoint.
// POST /api/v2/upload
//
// Multipart form field:
//
//	file — the file to upload (PDF/JPG/PNG, max 10 MB)
//
// Response 200:
//
//	{
//	  "success": true,
//	  "file_key": "abc123def456.pdf",
//	  "original_name": "فاتورة_الشراء.pdf",
//	  "file_size": 234567,
//	  "mime_type": "application/pdf",
//	  "download_url": "/api/v2/files/abc123def456.pdf"
//	}
//
// The frontend stores file_key or download_url and sends it later
// when creating/updating a purchase bill:
//
//	{ "pdf_link": "/api/v2/files/abc123def456.pdf", "attachments": [...] }
func (h *handler) UploadFile(c *gin.Context) {
	// Parse multipart
	if err := c.Request.ParseMultipartForm(MaxFileSize + 1024); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "فشل في قراءة بيانات الرفع"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "يرجى اختيار ملف للرفع"})
		return
	}
	defer file.Close()

	// Validate size
	if header.Size > MaxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"detail": fmt.Sprintf("حجم الملف يتجاوز الحد المسموح (%.0f ميجابايت)", float64(MaxFileSize)/(1<<20)),
		})
		return
	}

	// Validate extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedUploadExt[ext] {
		c.JSON(http.StatusBadRequest, gin.H{
			"detail": "صيغة الملف غير مدعومة. الصيغ المسموحة: PDF, JPG, PNG",
		})
		return
	}

	// Generate unique file key: 16 random hex bytes + extension
	fileKey := generateFileKey(ext)

	// Ensure upload directory exists
	if err := os.MkdirAll(FilesUploadDir, 0750); err != nil {
		log.Printf("[UPLOAD] Failed to create directory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "فشل في إنشاء مجلد الرفع"})
		return
	}

	// Save file to disk
	dstPath := filepath.Join(FilesUploadDir, fileKey)
	dst, err := os.Create(dstPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "فشل في حفظ الملف"})
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(dstPath)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "فشل في كتابة الملف"})
		return
	}

	mimeType := mimeTypes[ext]
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Record in database
	userID := GetSessionInfo(c)
	_, err = h.DB.Exec(`
		INSERT INTO uploaded_files (file_key, original_name, file_size, mime_type, uploaded_by)
		VALUES (?, ?, ?, ?, ?)
	`, fileKey, header.Filename, written, mimeType, userID.id)
	if err != nil {
		os.Remove(dstPath)
		log.Printf("[UPLOAD] Failed to insert file record: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "فشل في تسجيل الملف"})
		return
	}

	downloadURL := fmt.Sprintf("/api/v2/files/%s", fileKey)

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"file_key":      fileKey,
		"original_name": header.Filename,
		"file_size":     written,
		"mime_type":     mimeType,
		"download_url":  downloadURL,
	})
}

// DownloadFile serves an uploaded file by its key.
// GET /api/v2/files/:key
//
// Streams the file with the correct Content-Type.
func (h *handler) DownloadFile(c *gin.Context) {
	fileKey := c.Param("key")

	// Sanitize — prevent path traversal
	fileKey = filepath.Base(fileKey)
	if fileKey == "." || fileKey == ".." || strings.Contains(fileKey, "/") || strings.Contains(fileKey, "\\") {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "معرف ملف غير صالح"})
		return
	}

	// Look up in database
	var originalName string
	var mimeType sql.NullString
	err := h.DB.QueryRow(`
		SELECT original_name, mime_type FROM uploaded_files WHERE file_key = ?
	`, fileKey).Scan(&originalName, &mimeType)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "الملف غير موجود"})
		return
	}

	filePath := filepath.Join(FilesUploadDir, fileKey)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"detail": "الملف غير موجود على الخادم"})
		return
	}

	contentType := "application/octet-stream"
	if mimeType.Valid && mimeType.String != "" {
		contentType = mimeType.String
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", originalName))
	c.File(filePath)
}

// DeleteFile deletes an uploaded file by its key.
// DELETE /api/v2/files/:key
//
// Response 200: {"success": true, "message": "تم حذف الملف بنجاح"}
func (h *handler) DeleteFile(c *gin.Context) {
	fileKey := filepath.Base(c.Param("key"))

	result, err := h.DB.Exec(`DELETE FROM uploaded_files WHERE file_key = ?`, fileKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "فشل في حذف سجل الملف"})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "الملف غير موجود"})
		return
	}

	filePath := filepath.Join(FilesUploadDir, fileKey)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		log.Printf("[UPLOAD] Warning: failed to delete file from disk: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "تم حذف الملف بنجاح",
	})
}

// ══════════════════════════════════════════════════════════════════════════════
// PURCHASE BILL ATTACHMENT HELPERS
// ══════════════════════════════════════════════════════════════════════════════
//
// Flow:
//   1. Frontend uploads each file via POST /api/v2/upload → gets download_url per file
//   2. Frontend submits purchase bill with:
//        "pdf_link":    "/api/v2/files/abc123.pdf"        (mandatory)
//        "attachments": ["/api/v2/files/xyz789.pdf", ...] (optional)
//   3. Backend creates the purchase bill row (INSERT INTO purchase_bill)
//   4. Backend calls SavePurchaseBillAttachments(db, newBillID, pdfLink, attachments)
//        → Updates purchase_bill.pdf_link
//        → Inserts rows into purchase_bill_attachments join table
//   5. When fetching a bill detail, call GetPurchaseBillAttachments(db, billID)
//        → Returns the pdf_link + list of attachment URLs for the response JSON
//
// Tables involved:
//   uploaded_files             — generic file storage (populated by POST /api/v2/upload)
//   purchase_bill.pdf_link    — the mandatory bill PDF file_key
//   purchase_bill_attachments  — join table linking bill IDs to file_keys
//
// The FK constraint on purchase_bill_attachments.file_key → uploaded_files.file_key
// ensures that only previously-uploaded files can be linked to a bill.
// This is why the upload MUST succeed first (step 1) before the bill can reference it (step 4).

// SavePurchaseBillAttachments links uploaded files to a purchase bill after creation.
// Call this AFTER the bill row is inserted and you have the new bill ID.
//
// pdfLink: download URL like "/api/v2/files/abc123.pdf" (mandatory)
// attachments: array of download URLs (optional, can be empty)
//
// It extracts the file_key from each URL (last path segment) and:
//  1. Updates purchase_bill.pdf_link with the file_key
//  2. Inserts each attachment into purchase_bill_attachments
func (h *handler) SavePurchaseBillAttachments(db *sql.DB, billID int64, pdfLink string, attachments []string) error {
	// Extract file_key from URL: "/api/v2/files/abc123.pdf" → "abc123.pdf"
	pdfKey := filepath.Base(pdfLink)

	// Verify the file exists in uploaded_files before linking
	var exists int
	err := db.QueryRow("SELECT COUNT(*) FROM uploaded_files WHERE file_key = ?", pdfKey).Scan(&exists)
	if err != nil || exists == 0 {
		log.Printf("[ATTACHMENTS] pdf_link file_key %q not found in uploaded_files", pdfKey)
		return fmt.Errorf("ملف PDF غير موجود: %s", pdfKey)
	}

	// Update the purchase bill's pdf_link column
	_, err = db.Exec("UPDATE purchase_bill SET pdf_link = ? WHERE id = ?", pdfKey, billID)
	if err != nil {
		log.Printf("[ATTACHMENTS] Failed to update pdf_link for bill %d: %v", billID, err)
		return fmt.Errorf("فشل في ربط ملف PDF بالفاتورة: %v", err)
	}

	// Insert each attachment into the join table
	for _, att := range attachments {
		attKey := filepath.Base(att) // "/api/v2/files/xyz789.pdf" → "xyz789.pdf"

		// Verify the file exists first (FK will reject it anyway, but give a better error)
		err := db.QueryRow("SELECT COUNT(*) FROM uploaded_files WHERE file_key = ?", attKey).Scan(&exists)
		if err != nil || exists == 0 {
			log.Printf("[ATTACHMENTS] attachment file_key %q not found in uploaded_files, skipping", attKey)
			continue
		}

		_, err = db.Exec(
			"INSERT IGNORE INTO purchase_bill_attachments (purchase_bill_id, file_key) VALUES (?, ?)",
			billID, attKey,
		)
		if err != nil {
			log.Printf("[ATTACHMENTS] Failed to insert attachment %s for bill %d: %v", attKey, billID, err)
			// Don't fail the whole operation for optional attachments
		}
	}

	log.Printf("[ATTACHMENTS] Linked bill %d: pdf=%s, %d attachments", billID, pdfKey, len(attachments))
	return nil
}

// GetPurchaseBillAttachments retrieves file references for a purchase bill.
// Returns pdfLink (as download URL) and attachments (as download URL array).
// Call this in your GetPurchaseBillById handler to include files in the response.
func (h *handler) GetPurchaseBillAttachments(db *sql.DB, billID int64) (pdfLink string, attachments []string) {
	// Get pdf_link from the purchase_bill table
	var pdfKey sql.NullString
	err := db.QueryRow("SELECT pdf_link FROM purchase_bill WHERE id = ?", billID).Scan(&pdfKey)
	if err != nil {
		log.Printf("[ATTACHMENTS] Failed to get pdf_link for bill %d: %v", billID, err)
		return "", nil
	}
	if pdfKey.Valid && pdfKey.String != "" {
		pdfLink = "/api/v2/files/" + pdfKey.String
	}

	// Get attachments from the join table
	rows, err := db.Query(
		"SELECT file_key FROM purchase_bill_attachments WHERE purchase_bill_id = ? ORDER BY created_at",
		billID,
	)
	if err != nil {
		log.Printf("[ATTACHMENTS] Failed to get attachments for bill %d: %v", billID, err)
		return pdfLink, nil
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err == nil {
			attachments = append(attachments, "/api/v2/files/"+key)
		}
	}

	return pdfLink, attachments
}

// DeletePurchaseBillAttachments removes all attachment links for a purchase bill.
// Call this before deleting a bill (CASCADE handles it too, but explicit is cleaner),
// or when updating a bill's attachments (delete old → insert new).
func (h *handler) DeletePurchaseBillAttachments(db *sql.DB, billID int64) error {
	_, err := db.Exec("DELETE FROM purchase_bill_attachments WHERE purchase_bill_id = ?", billID)
	if err != nil {
		log.Printf("[ATTACHMENTS] Failed to delete attachments for bill %d: %v", billID, err)
	}
	return err
}

// UpdatePurchaseBillAttachments replaces all attachments for a bill.
// Deletes existing attachments then saves new ones.
// Call this in your UpdatePurchaseBill handler.
func (h *handler) UpdatePurchaseBillAttachments(db *sql.DB, billID int64, pdfLink string, attachments []string) error {
	// Delete existing attachments
	if err := h.DeletePurchaseBillAttachments(db, billID); err != nil {
		return err
	}
	// Save new ones
	return h.SavePurchaseBillAttachments(db, billID, pdfLink, attachments)
}

// ══════════════════════════════════════════════════════════════════════════════
// INTEGRATION EXAMPLE — How to wire into your purchase bill handlers
// ══════════════════════════════════════════════════════════════════════════════
//
// ── In AddPurchaseBill (create) handler: ─────────────────────────────────────
//
//   // After INSERT INTO purchase_bill → get newBillID
//   if req.PDFLink != "" {
//       if err := h.SavePurchaseBillAttachments(h.DB, newBillID, req.PDFLink, req.Attachments); err != nil {
//           // Bill was created but files couldn't be linked — log but don't fail
//           log.Printf("Warning: bill %d created but attachment linking failed: %v", newBillID, err)
//       }
//   }
//
// ── In UpdatePurchaseBill handler: ───────────────────────────────────────────
//
//   // After UPDATE purchase_bill SET ... WHERE id = billID
//   if req.PDFLink != "" {
//       if err := h.UpdatePurchaseBillAttachments(h.DB, billID, req.PDFLink, req.Attachments); err != nil {
//           log.Printf("Warning: bill %d updated but attachment linking failed: %v", billID, err)
//       }
//   }
//
// ── In GetPurchaseBillById handler: ──────────────────────────────────────────
//
//   // After fetching the bill from DB
//   pdfLink, attachments := h.GetPurchaseBillAttachments(h.DB, billID)
//   response["pdf_link"] = pdfLink
//   response["attachments"] = attachments
//
// ── In DeletePurchaseBill handler: ───────────────────────────────────────────
//
//   // CASCADE on FK handles this automatically, but you could also:
//   h.DeletePurchaseBillAttachments(h.DB, billID)
//   // Then DELETE FROM purchase_bill WHERE id = billID
//

// ── Helpers ─────────────────────────────────────────────────────────────────

// generateFileKey creates a unique file key: 16 random hex bytes + extension.
// Example: "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6.pdf"
func generateFileKey(ext string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	}
	return hex.EncodeToString(b) + ext
}

// getUserIDFromContext extracts the user ID from JWT claims set by middleware.
func getUserIDFromContext(c *gin.Context) int64 {
	if id, exists := c.Get("userId"); exists {
		switch v := id.(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		}
	}
	return 0
}

// ── Cleanup ─────────────────────────────────────────────────────────────────

// CleanupOrphanFiles deletes uploaded files that aren't referenced by any
// purchase bill. Run periodically (e.g. daily cron) to prevent disk bloat.
//
//	DELETE uf FROM uploaded_files uf
//	LEFT JOIN purchase_bill pb ON pb.pdf_link = CONCAT('/api/v2/files/', uf.file_key)
//	LEFT JOIN purchase_bill_attachments pba ON pba.file_key = uf.file_key
//	WHERE pb.id IS NULL AND pba.id IS NULL
//	  AND uf.created_at < NOW() - INTERVAL 24 HOUR;
func (h *handler) CleanupOrphanFiles() {
	rows, err := h.DB.Query(`
		SELECT uf.file_key FROM uploaded_files uf
		LEFT JOIN purchase_bill pb ON pb.pdf_link = uf.file_key
		LEFT JOIN purchase_bill_attachments pba ON pba.file_key = uf.file_key
		WHERE pb.id IS NULL AND pba.id IS NULL
		  AND uf.created_at < NOW() - INTERVAL 24 HOUR
	`)
	if err != nil {
		log.Printf("[CLEANUP] Query error: %v", err)
		return
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err == nil {
			keys = append(keys, key)
		}
	}

	for _, key := range keys {
		filePath := filepath.Join(FilesUploadDir, key)
		os.Remove(filePath)
		h.DB.Exec("DELETE FROM uploaded_files WHERE file_key = ?", key)
	}

	if len(keys) > 0 {
		log.Printf("[CLEANUP] Removed %d orphaned files", len(keys))
	}
}
