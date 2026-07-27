-- 0008_all_table_timestamps.sql
-- Add consistent audit timestamps to every application table.
--
-- The canonical schema declares these columns for fresh databases. This
-- migration is idempotent for existing databases and preserves the value
-- of an existing timestamp when its companion column is introduced.

CREATE TEMPORARY TABLE IF NOT EXISTS `_timestamp_migration_state` (
  `table_name` varchar(64) NOT NULL PRIMARY KEY,
  `table_exists` tinyint NOT NULL,
  `has_created_at` tinyint NOT NULL,
  `has_updated_at` tinyint NOT NULL
) ENGINE=MEMORY;

INSERT INTO `_timestamp_migration_state` (`table_name`, `table_exists`, `has_created_at`, `has_updated_at`)
SELECT t.`table_name`,
       1,
       COALESCE(MAX(c.`column_name` = 'created_at'), 0),
       COALESCE(MAX(c.`column_name` = 'updated_at'), 0)
  FROM information_schema.tables t
  LEFT JOIN information_schema.columns c
    ON c.`table_schema` = t.`table_schema`
   AND c.`table_name` = t.`table_name`
 WHERE t.`table_schema` = DATABASE()
   AND t.`table_type` = 'BASE TABLE'
 GROUP BY t.`table_name`
ON DUPLICATE KEY UPDATE
  `table_exists` = VALUES(`table_exists`),
  `has_created_at` = VALUES(`has_created_at`),
  `has_updated_at` = VALUES(`has_updated_at`);

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'account'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'account'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'account'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `account`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'ambrand'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'ambrand'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'ambrand'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `ambrand`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'ambrandsaddress'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'ambrandsaddress'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'ambrandsaddress'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `ambrandsaddress`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'article_car'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'article_car'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'article_car'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `article_car`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'article_car_link'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'article_car_link'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'article_car_link'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `article_car_link`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlecriteria'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlecriteria'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlecriteria'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `articlecriteria`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlecrosses'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlecrosses'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlecrosses'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `articlecrosses`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'articledocs'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articledocs'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articledocs'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `articledocs`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'articleean'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articleean'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articleean'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `articleean`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlelinks'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlelinks'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlelinks'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `articlelinks`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlemain'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlemain'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlemain'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `articlemain`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlepdfs'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlepdfs'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlepdfs'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `articlepdfs`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'articles'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articles'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articles'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `articles`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlesvehicletrees'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlesvehicletrees'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articlesvehicletrees'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `articlesvehicletrees`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'articletext'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articletext'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'articletext'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `articletext`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'assemblygroupnodenames'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'assemblygroupnodenames'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'assemblygroupnodenames'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `assemblygroupnodenames`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'assemblygroupnodes'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'assemblygroupnodes'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'assemblygroupnodes'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `assemblygroupnodes`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'axlebodytype'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'axlebodytype'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'axlebodytype'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `axlebodytype`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'axlebrakesizes'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'axlebrakesizes'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'axlebrakesizes'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `axlebrakesizes`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'axledetails'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'axledetails'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'axledetails'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `axledetails`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'axles'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'axles'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'axles'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `axles`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'bill'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'bill'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'bill'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `bill`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'bill_payment'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'bill_payment'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'bill_payment'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `bill_payment`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'bill_product'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'bill_product'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'bill_product'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `bill_product`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'bodymark'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'bodymark'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'bodymark'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `bodymark`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'bodymarkcarids'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'bodymarkcarids'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'bodymarkcarids'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `bodymarkcarids`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'branch_zatca_config'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'branch_zatca_config'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'branch_zatca_config'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `branch_zatca_config`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'branches'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'branches'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'branches'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `branches`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'car_link'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'car_link'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'car_link'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `car_link`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'cars'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'cars'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'cars'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `cars`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'cars_old'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'cars_old'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'cars_old'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `cars_old`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'carsbodies'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'carsbodies'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'carsbodies'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `carsbodies`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'cash_voucher'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'cash_voucher'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'cash_voucher'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `cash_voucher`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'client'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'client'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'client'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `client`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'company'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'company'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'company'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `company`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'countries'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'countries'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'countries'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `countries`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'countrygroups'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'countrygroups'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'countrygroups'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `countrygroups`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'credit_note'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'credit_note'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'credit_note'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `credit_note`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'criteria'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'criteria'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'criteria'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `criteria`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'dashboard_daily_rollup'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'dashboard_daily_rollup'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'dashboard_daily_rollup'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `dashboard_daily_rollup`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'debit_note'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'debit_note'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'debit_note'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `debit_note`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'expense'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'expense'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'expense'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `expense`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'expense_category'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'expense_category'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'expense_category'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `expense_category`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'genericarticles'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'genericarticles'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'genericarticles'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `genericarticles`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'genericarticlesgroups'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'genericarticlesgroups'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'genericarticlesgroups'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `genericarticlesgroups`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'journal_entry'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'journal_entry'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'journal_entry'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `journal_entry`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'journal_line'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'journal_line'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'journal_line'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `journal_line`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'keyvalues'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'keyvalues'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'keyvalues'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `keyvalues`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'languages'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'languages'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'languages'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `languages`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'legacy2generic'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'legacy2generic'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'legacy2generic'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `legacy2generic`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'linkagetargets'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'linkagetargets'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'linkagetargets'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `linkagetargets`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'manufacturermotorids'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'manufacturermotorids'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'manufacturermotorids'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `manufacturermotorids`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'manufacturers'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'manufacturers'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'manufacturers'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `manufacturers`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'modelseries'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'modelseries'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'modelseries'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `modelseries`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'modelseries_old'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'modelseries_old'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'modelseries_old'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `modelseries_old`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'motordetails'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'motordetails'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'motordetails'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `motordetails`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'newarticles'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'newarticles'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'newarticles'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `newarticles`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'notification_settings'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'notification_settings'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'notification_settings'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `notification_settings`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'notifications'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'notifications'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'notifications'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `notifications`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'oem_number'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'oem_number'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'oem_number'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `oem_number`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'oemnumbers'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'oemnumbers'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'oemnumbers'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `oemnumbers`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'order_items'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'order_items'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'order_items'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `order_items`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'orders'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'orders'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'orders'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `orders`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'product'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'product'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'product'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `product`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'purchase_bill'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'purchase_bill'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'purchase_bill'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `purchase_bill`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'purchase_bill_attachments'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'purchase_bill_attachments'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'purchase_bill_attachments'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `purchase_bill_attachments`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'purchase_bill_payment'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'purchase_bill_payment'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'purchase_bill_payment'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `purchase_bill_payment`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'purchase_bill_product'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'purchase_bill_product'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'purchase_bill_product'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `purchase_bill_product`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'refresh_token'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'refresh_token'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'refresh_token'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `refresh_token`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'replacedbyarticles'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'replacedbyarticles'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'replacedbyarticles'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `replacedbyarticles`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'replacesarticles'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'replacesarticles'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'replacesarticles'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `replacesarticles`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'searchindex'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'searchindex'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'searchindex'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `searchindex`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'settings'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'settings'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'settings'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `settings`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'shortcuts'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'shortcuts'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'shortcuts'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `shortcuts`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'stock_movements'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'stock_movements'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'stock_movements'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `stock_movements`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'store'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'store'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'store'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `store`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'supplier'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'supplier'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'supplier'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `supplier`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'tradenumbers'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'tradenumbers'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'tradenumbers'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `tradenumbers`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'uploaded_files'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'uploaded_files'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'uploaded_files'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `uploaded_files`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'user'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'user'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'user'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `user`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'user_permission'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'user_permission'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'user_permission'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `user_permission`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehicleaxles'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehicleaxles'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehicleaxles'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `vehicleaxles`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehicledetails'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehicledetails'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehicledetails'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `vehicledetails`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehiclemotorcodes'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehiclemotorcodes'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehiclemotorcodes'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `vehiclemotorcodes`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehicleprototypes'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehicleprototypes'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehicleprototypes'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `vehicleprototypes`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehiclesecondarytypes'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehiclesecondarytypes'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehiclesecondarytypes'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `vehiclesecondarytypes`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehicletrees'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehicletrees'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehicletrees'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `vehicletrees`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehiclewheelbases'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehiclewheelbases'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vehiclewheelbases'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `vehiclewheelbases`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'vin_cache'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vin_cache'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'vin_cache'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `vin_cache`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'zatca_submission'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'zatca_submission'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'zatca_submission'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `zatca_submission`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'password_reset_tokens'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'password_reset_tokens'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'password_reset_tokens'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `password_reset_tokens`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'sessions'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'sessions'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'sessions'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `sessions`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_exists := COALESCE((SELECT `table_exists` FROM `_timestamp_migration_state` WHERE `table_name` = 'branch_sequence'), 0);
SET @timestamp_created := COALESCE((SELECT `has_created_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'branch_sequence'), 0);
SET @timestamp_updated := COALESCE((SELECT `has_updated_at` FROM `_timestamp_migration_state` WHERE `table_name` = 'branch_sequence'), 0);
SET @timestamp_add_created := IF(@timestamp_created = 0, IF(@timestamp_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @timestamp_add_updated := IF(@timestamp_updated = 0, IF(@timestamp_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @timestamp_separator := IF(@timestamp_add_created <> '' AND @timestamp_add_updated <> '', ',', '');
SET @timestamp_sql := IF(@timestamp_exists = 1 AND (@timestamp_add_created <> '' OR @timestamp_add_updated <> ''), CONCAT('ALTER TABLE `branch_sequence`', @timestamp_add_created, @timestamp_separator, @timestamp_add_updated), 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'bill_payment' AND `has_created_at` = 1 AND `has_updated_at` = 0),
  'UPDATE `bill_payment` SET `updated_at` = `created_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'credit_note' AND `has_created_at` = 1 AND `has_updated_at` = 0),
  'UPDATE `credit_note` SET `updated_at` = `created_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'debit_note' AND `has_created_at` = 1 AND `has_updated_at` = 0),
  'UPDATE `debit_note` SET `updated_at` = `created_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'expense' AND `has_created_at` = 1 AND `has_updated_at` = 0),
  'UPDATE `expense` SET `updated_at` = `created_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'journal_entry' AND `has_created_at` = 1 AND `has_updated_at` = 0),
  'UPDATE `journal_entry` SET `updated_at` = `created_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'notifications' AND `has_created_at` = 1 AND `has_updated_at` = 0),
  'UPDATE `notifications` SET `updated_at` = `created_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'order_items' AND `has_created_at` = 1 AND `has_updated_at` = 0),
  'UPDATE `order_items` SET `updated_at` = `created_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'purchase_bill_attachments' AND `has_created_at` = 1 AND `has_updated_at` = 0),
  'UPDATE `purchase_bill_attachments` SET `updated_at` = `created_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'purchase_bill_payment' AND `has_created_at` = 1 AND `has_updated_at` = 0),
  'UPDATE `purchase_bill_payment` SET `updated_at` = `created_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'refresh_token' AND `has_created_at` = 1 AND `has_updated_at` = 0),
  'UPDATE `refresh_token` SET `updated_at` = `created_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'stock_movements' AND `has_created_at` = 1 AND `has_updated_at` = 0),
  'UPDATE `stock_movements` SET `updated_at` = `created_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'uploaded_files' AND `has_created_at` = 1 AND `has_updated_at` = 0),
  'UPDATE `uploaded_files` SET `updated_at` = `created_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'password_reset_tokens' AND `has_created_at` = 1 AND `has_updated_at` = 0),
  'UPDATE `password_reset_tokens` SET `updated_at` = `created_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'sessions' AND `has_created_at` = 1 AND `has_updated_at` = 0),
  'UPDATE `sessions` SET `updated_at` = `created_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'settings' AND `has_created_at` = 0 AND `has_updated_at` = 1),
  'UPDATE `settings` SET `created_at` = `updated_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

SET @timestamp_sql := IF(EXISTS (SELECT 1 FROM `_timestamp_migration_state` WHERE `table_name` = 'branch_sequence' AND `has_created_at` = 0 AND `has_updated_at` = 1),
  'UPDATE `branch_sequence` SET `created_at` = `updated_at`', 'DO 0');
PREPARE timestamp_stmt FROM @timestamp_sql;
EXECUTE timestamp_stmt;
DEALLOCATE PREPARE timestamp_stmt;

-- car_part is a separate optional database and is not visible through
-- DATABASE(). Add its audit columns only when that database is installed.
SET @car_part_exists := (
  SELECT COUNT(*) FROM information_schema.tables
   WHERE table_schema = 'car_part' AND table_name = 'oem_number'
);
SET @car_part_created := (
  SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = 'car_part' AND table_name = 'oem_number'
     AND column_name = 'created_at'
);
SET @car_part_updated := (
  SELECT COUNT(*) FROM information_schema.columns
   WHERE table_schema = 'car_part' AND table_name = 'oem_number'
     AND column_name = 'updated_at'
);
SET @car_part_add_created := IF(@car_part_created = 0, IF(@car_part_updated = 1, ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER `updated_at`', ' ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP'), '');
SET @car_part_add_updated := IF(@car_part_updated = 0, IF(@car_part_created = 1, ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`', ' ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP'), '');
SET @car_part_separator := IF(@car_part_add_created <> '' AND @car_part_add_updated <> '', ',', '');
SET @car_part_sql := IF(@car_part_exists = 1 AND (@car_part_add_created <> '' OR @car_part_add_updated <> ''),
  CONCAT('ALTER TABLE `car_part`.`oem_number`', @car_part_add_created, @car_part_separator, @car_part_add_updated),
  'DO 0');
PREPARE car_part_stmt FROM @car_part_sql;
EXECUTE car_part_stmt;
DEALLOCATE PREPARE car_part_stmt;
SET @car_part_sql := IF(@car_part_exists = 1 AND @car_part_created = 1 AND @car_part_updated = 0,
  'UPDATE `car_part`.`oem_number` SET `updated_at` = `created_at`',
  IF(@car_part_exists = 1 AND @car_part_created = 0 AND @car_part_updated = 1,
     'UPDATE `car_part`.`oem_number` SET `created_at` = `updated_at`',
     'DO 0'));
PREPARE car_part_stmt FROM @car_part_sql;
EXECUTE car_part_stmt;
DEALLOCATE PREPARE car_part_stmt;

DROP TEMPORARY TABLE IF EXISTS `_timestamp_migration_state`;
