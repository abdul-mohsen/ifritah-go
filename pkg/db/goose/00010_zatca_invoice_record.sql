-- +goose Up
-- The invoice record (D25). The daemon commits a signed invoice with its ICV and its hash in one transaction, and the signed
-- XML never changes after that, so it is stored here instead of in a file. The chain columns stay NULL until the invoice is
-- committed. The old file path of the signed XML is removed, because nothing writes it.
ALTER TABLE zatca_submission
  ADD COLUMN branch_id INT UNSIGNED NULL,
  ADD COLUMN invoice_icv BIGINT UNSIGNED NULL,
  ADD COLUMN previous_invoice_hash VARCHAR(255) NULL,
  ADD COLUMN invoice_uuid CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN invoice_hash VARCHAR(128) NULL,
  ADD COLUMN invoice_kind VARCHAR(16) NULL,
  ADD COLUMN signed_xml MEDIUMTEXT NULL,
  ADD UNIQUE KEY uq_zatca_invoice_chain (branch_id, invoice_icv),
  ADD UNIQUE KEY uq_zatca_invoice_uuid (invoice_uuid),
  ADD CONSTRAINT chk_zatca_invoice_kind CHECK (invoice_kind IN ('standard', 'simplified') OR invoice_kind IS NULL),
  DROP COLUMN request_xml_path;

ALTER TABLE bill
  DROP COLUMN invoice_xml_path;

-- +goose Down
ALTER TABLE bill
  ADD COLUMN invoice_xml_path VARCHAR(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL
    COMMENT 'Filesystem path of signed XML (relative to xml-dir)';

ALTER TABLE zatca_submission
  DROP CHECK chk_zatca_invoice_kind,
  DROP INDEX uq_zatca_invoice_uuid,
  DROP INDEX uq_zatca_invoice_chain,
  DROP COLUMN signed_xml,
  DROP COLUMN invoice_kind,
  DROP COLUMN invoice_hash,
  DROP COLUMN invoice_uuid,
  DROP COLUMN previous_invoice_hash,
  DROP COLUMN invoice_icv,
  DROP COLUMN branch_id,
  ADD COLUMN request_xml_path VARCHAR(500) NULL;
