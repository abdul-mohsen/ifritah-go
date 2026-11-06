-- +goose Up
-- A payment needs a currency. The baseline has bill_payment.currency_id, but no table defines the currencies, so this
-- migration creates one and seeds SAR, the only currency the backend writes. The sale path looks SAR up by code, so no id is
-- hard-coded. There is no foreign key: a bill_payment row written before this migration may hold any currency_id.
CREATE TABLE currency (
  id INT NOT NULL AUTO_INCREMENT,
  code CHAR(3) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_currency_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO currency (code, name) VALUES ('SAR', 'Saudi riyal');

-- +goose Down
DROP TABLE IF EXISTS currency;
