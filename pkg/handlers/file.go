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
// -- Then modify purchase_bills to include file references:
// ALTER TABLE purchase_bills
//   ADD COLUMN pdf_link     VARCHAR(255) NULL COMMENT 'file_key of the mandatory bill PDF';
//
// -- Optional: a join table for multiple attachments per bill.
// -- Or just store a JSON array in purchase_bills.attachments TEXT column.
// -- Option A: JSON column (simpler)
// ALTER TABLE purchase_bills
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
//     REFERENCES purchase_bills(id) ON DELETE CASCADE,
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
	userID := getUserIDFromContext(c)
	_, err = h.DB.Exec(`
		INSERT INTO uploaded_files (file_key, original_name, file_size, mime_type, uploaded_by)
		VALUES (?, ?, ?, ?, ?)
	`, fileKey, header.Filename, written, mimeType, userID)
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
// PURCHASE BILL INTEGRATION
// ══════════════════════════════════════════════════════════════════════════════
//
// When creating/updating a purchase bill, the payload now includes:
//
//   {
//     "store_id": 1,
//     "merchant_id": 5,
//     "supplier_id": 5,
//     "supplier_sequence_number": 1001,
//     "effective_date": "2025-01-15",
//     "products": [...],
//     "discount": "5",
//     "subtotal": 500.0,
//     "pdf_link": "/api/v2/files/abc123def456.pdf",       ← NEW (mandatory)
//     "attachments": [                                     ← NEW (optional)
//       "/api/v2/files/789ghi012jkl.pdf",
//       "/api/v2/files/mno345pqr678.jpg"
//     ]
//   }
//
// In your existing AddPurchaseBill handler, add this after creating the bill:
//
//   // After INSERT INTO purchase_bills ... → get newBillID
//
//   // Save pdf_link (mandatory)
//   if req.PDFLink != "" {
//       db.Exec("UPDATE purchase_bills SET pdf_link = ? WHERE id = ?", req.PDFLink, newBillID)
//   }
//
//   // Save attachments (optional) — using join table approach
//   for _, att := range req.Attachments {
//       fileKey := filepath.Base(att) // extract key from URL
//       db.Exec(`INSERT INTO purchase_bill_attachments (purchase_bill_id, file_key) VALUES (?, ?)`,
//           newBillID, fileKey)
//   }
//
// In your GetPurchaseBillById handler, include the file data in the response:
//
//   // After fetching the bill...
//   bill.PDFLink = "/api/v2/files/" + bill.PDFLinkKey
//   rows, _ := db.Query("SELECT file_key FROM purchase_bill_attachments WHERE purchase_bill_id = ?", billID)
//   for rows.Next() {
//       var key string
//       rows.Scan(&key)
//       bill.Attachments = append(bill.Attachments, "/api/v2/files/" + key)
//   }
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
//	LEFT JOIN purchase_bills pb ON pb.pdf_link = CONCAT('/api/v2/files/', uf.file_key)
//	LEFT JOIN purchase_bill_attachments pba ON pba.file_key = uf.file_key
//	WHERE pb.id IS NULL AND pba.id IS NULL
//	  AND uf.created_at < NOW() - INTERVAL 24 HOUR;
func (h *handler) CleanupOrphanFiles() {
	rows, err := h.DB.Query(`
		SELECT uf.file_key FROM uploaded_files uf
		LEFT JOIN purchase_bills pb ON pb.pdf_link = uf.file_key
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
