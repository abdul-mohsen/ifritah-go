-- +goose Up
-- Credit and debit notes (B6, D26 to D29). A note is a final document, like a bill: it has the same
-- request identity and fingerprint (D11, D21), its invoice UUID is created by the backend (D14), and it
-- writes its ZATCA record to the same chain as the bills.
--
-- zatca_submission keeps one invoice record for every document kind, keyed by (document_type,
-- source_id). bill_id still points at the original bill for every kind, so a note points at the bill
-- it credits or charges. The existing rows are bills, and the source of a bill is the bill itself.
ALTER TABLE zatca_submission
  ADD COLUMN document_type VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'bill',
  ADD COLUMN source_id BIGINT UNSIGNED NULL;

UPDATE zatca_submission SET source_id = bill_id;

ALTER TABLE zatca_submission
  MODIFY COLUMN source_id BIGINT UNSIGNED NOT NULL,
  ADD KEY idx_zatca_bill (bill_id),
  ADD CONSTRAINT chk_zatca_document_type CHECK (document_type IN ('bill', 'credit', 'debit')),
  ADD CONSTRAINT chk_zatca_bill_source CHECK (document_type <> 'bill' OR source_id = bill_id);

ALTER TABLE zatca_submission
  DROP KEY uq_zatca_bill,
  ADD UNIQUE KEY uq_zatca_document (document_type, source_id);

-- A bill can have more than one credit note, and more than one debit note. The credit note that
-- existed before this migration came from the old flow, which credited the whole bill, so its amount
-- is the bill total.
ALTER TABLE credit_note
  DROP KEY unique_bill_id,
  ADD COLUMN amount DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT 'Credited total, with VAT',
  ADD COLUMN lines_total DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT 'Sum of the lines, with VAT',
  ADD COLUMN pos_id VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NULL,
  ADD COLUMN bill_request_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN request_fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN request_fingerprint_version SMALLINT UNSIGNED NOT NULL DEFAULT 1,
  ADD UNIQUE KEY uq_credit_note_request (pos_id, bill_request_id),
  ADD UNIQUE KEY uq_credit_note_invoice_uuid (invoice_uuid),
  ADD CONSTRAINT chk_credit_note_request_fingerprint CHECK (request_fingerprint IS NULL OR request_fingerprint REGEXP '^[0-9a-f]{64}$');

UPDATE credit_note cn
  JOIN bill b ON b.id = cn.bill_id
SET cn.amount = b.total, cn.lines_total = b.total;

-- The index on bill_id is added before the unique key is dropped, because the foreign key needs it.
ALTER TABLE debit_note
  ADD KEY idx_dn_bill_id (bill_id);

ALTER TABLE debit_note
  DROP KEY uq_dn_bill_id,
  ADD COLUMN amount DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT 'Added total, with VAT',
  ADD COLUMN lines_total DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT 'Sum of the lines, with VAT',
  ADD COLUMN pos_id VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NULL,
  ADD COLUMN bill_request_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN request_fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN request_fingerprint_version SMALLINT UNSIGNED NOT NULL DEFAULT 1,
  ADD UNIQUE KEY uq_debit_note_request (pos_id, bill_request_id),
  ADD UNIQUE KEY uq_debit_note_invoice_uuid (invoice_uuid),
  ADD CONSTRAINT chk_debit_note_request_fingerprint CHECK (request_fingerprint IS NULL OR request_fingerprint REGEXP '^[0-9a-f]{64}$');

-- The lines of a credit note. A line credits a line of the original bill, so bill_product_id is
-- that line, and the totals use the same rounding as bill_product. discount is the part of the
-- original line's discount that this quantity carries.
CREATE TABLE credit_note_line (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  credit_note_id BIGINT UNSIGNED NOT NULL,
  bill_product_id INT NOT NULL,
  name VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL,
  quantity DECIMAL(10,3) NOT NULL,
  price DECIMAL(12,2) NOT NULL,
  discount DECIMAL(12,2) NOT NULL DEFAULT 0.00,
  vat DECIMAL(5,2) NOT NULL,
  total_before_discount DECIMAL(12,2) GENERATED ALWAYS AS (ROUND((price * quantity), 2)) STORED,
  total_before_vat DECIMAL(12,2) GENERATED ALWAYS AS (ROUND((total_before_discount - discount), 2)) STORED,
  vat_total DECIMAL(12,2) GENERATED ALWAYS AS (ROUND(((total_before_vat * vat) / 100), 2)) STORED,
  total_including_vat DECIMAL(12,2) GENERATED ALWAYS AS (ROUND((total_before_vat + vat_total), 2)) STORED,
  PRIMARY KEY (id),
  KEY idx_cnl_note (credit_note_id),
  KEY idx_cnl_bill_product (bill_product_id),
  CONSTRAINT fk_cnl_note FOREIGN KEY (credit_note_id) REFERENCES credit_note (id) ON DELETE CASCADE,
  CONSTRAINT fk_cnl_bill_product FOREIGN KEY (bill_product_id) REFERENCES bill_product (id),
  CONSTRAINT chk_cnl_quantity CHECK (quantity > 0),
  CONSTRAINT chk_cnl_price CHECK (price > 0),
  CONSTRAINT chk_cnl_discount CHECK (discount >= 0),
  CONSTRAINT chk_cnl_vat CHECK (vat >= 0 AND vat <= 100)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- The lines of a debit note. A debit line is a new charge, so it has a name and no bill line.
CREATE TABLE debit_note_line (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  debit_note_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  quantity DECIMAL(10,3) NOT NULL,
  price DECIMAL(12,2) NOT NULL,
  vat DECIMAL(5,2) NOT NULL,
  total_before_vat DECIMAL(12,2) GENERATED ALWAYS AS (ROUND((price * quantity), 2)) STORED,
  vat_total DECIMAL(12,2) GENERATED ALWAYS AS (ROUND(((total_before_vat * vat) / 100), 2)) STORED,
  total_including_vat DECIMAL(12,2) GENERATED ALWAYS AS (ROUND((total_before_vat + vat_total), 2)) STORED,
  PRIMARY KEY (id),
  KEY idx_dnl_note (debit_note_id),
  CONSTRAINT fk_dnl_note FOREIGN KEY (debit_note_id) REFERENCES debit_note (id) ON DELETE CASCADE,
  CONSTRAINT chk_dnl_quantity CHECK (quantity > 0),
  CONSTRAINT chk_dnl_price CHECK (price > 0),
  CONSTRAINT chk_dnl_vat CHECK (vat >= 0 AND vat <= 100)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
-- Each guard runs before anything is dropped, so a refused rollback changes nothing. The first guard
-- refuses while a credit or debit note has a ZATCA record, because the old shape has no source key
-- for it. The unique keys of the old shape refuse a bill that has two credit notes or two debit notes.
ALTER TABLE zatca_submission ADD CONSTRAINT chk_zatca_down_bills_only CHECK (document_type = 'bill');
ALTER TABLE zatca_submission DROP CHECK chk_zatca_down_bills_only;

ALTER TABLE credit_note ADD UNIQUE KEY unique_bill_id (bill_id);
ALTER TABLE debit_note ADD UNIQUE KEY uq_dn_bill_id (bill_id);

ALTER TABLE zatca_submission ADD UNIQUE KEY uq_zatca_bill (bill_id);

ALTER TABLE zatca_submission
  DROP CHECK chk_zatca_bill_source,
  DROP CHECK chk_zatca_document_type,
  DROP KEY uq_zatca_document,
  DROP KEY idx_zatca_bill;

ALTER TABLE zatca_submission
  DROP COLUMN source_id,
  DROP COLUMN document_type;

DROP TABLE credit_note_line;
DROP TABLE debit_note_line;

ALTER TABLE credit_note
  DROP KEY uq_credit_note_invoice_uuid,
  DROP KEY uq_credit_note_request,
  DROP CHECK chk_credit_note_request_fingerprint;

ALTER TABLE credit_note
  DROP COLUMN request_fingerprint_version,
  DROP COLUMN request_fingerprint,
  DROP COLUMN bill_request_id,
  DROP COLUMN pos_id,
  DROP COLUMN lines_total,
  DROP COLUMN amount;

ALTER TABLE debit_note
  DROP KEY uq_debit_note_invoice_uuid,
  DROP KEY uq_debit_note_request,
  DROP CHECK chk_debit_note_request_fingerprint,
  DROP KEY idx_dn_bill_id;

ALTER TABLE debit_note
  DROP COLUMN request_fingerprint_version,
  DROP COLUMN request_fingerprint,
  DROP COLUMN bill_request_id,
  DROP COLUMN pos_id,
  DROP COLUMN lines_total,
  DROP COLUMN amount;
