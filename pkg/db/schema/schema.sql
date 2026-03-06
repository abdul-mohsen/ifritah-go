-- MySQL dump 10.13  Distrib 8.0.44, for Linux (x86_64)
--
-- Host: localhost    Database: dev_ifritah
-- ------------------------------------------------------
-- Server version	8.0.44

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!50503 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `ambrand`
--

DROP TABLE IF EXISTS `ambrand`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `ambrand` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `brandId` bigint DEFAULT NULL,
  `brandLogoID` varchar(250) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci DEFAULT NULL,
  `brandName` varchar(250) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci DEFAULT NULL,
  `lang` varchar(5) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci DEFAULT NULL COMMENT 'hide',
  `articleCountry` varchar(50) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `amBrand_id_uindex` (`id`) USING BTREE,
  KEY `brandId` (`brandId`) USING BTREE,
  KEY `brandLogoID` (`brandLogoID`) USING BTREE,
  KEY `lang` (`lang`) USING BTREE,
  KEY `brandName` (`brandName`) USING BTREE,
  KEY `articleCountry` (`articleCountry`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=35086 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Description of all aftermarket brands';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `ambrandsaddress`
--

DROP TABLE IF EXISTS `ambrandsaddress`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `ambrandsaddress` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `addressName` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `addressType` bigint DEFAULT NULL,
  `city` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `city2` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `fax` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `logoDocId` int DEFAULT NULL,
  `name` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `phone` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `street` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `wwwURL` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `zip` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `zipCountryCode` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `lang` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `brandId` bigint DEFAULT NULL,
  `email` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `name2` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `zipMailbox` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `zipSpecial` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `mailbox` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `street2` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=38299 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Main address of the data supplier';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `article_car`
--

DROP TABLE IF EXISTS `article_car`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `article_car` (
  `vehicleModelSeriesId` bigint NOT NULL,
  `legacyArticleId` bigint NOT NULL COMMENT 'legacyArticleId',
  PRIMARY KEY (`vehicleModelSeriesId`,`legacyArticleId`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `article_car_link`
--

DROP TABLE IF EXISTS `article_car_link`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `article_car_link` (
  `linkingTargetId` bigint NOT NULL,
  `legacyArticleId` bigint NOT NULL COMMENT 'legacyArticleId',
  PRIMARY KEY (`linkingTargetId`,`legacyArticleId`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `articlecriteria`
--

DROP TABLE IF EXISTS `articlecriteria`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `articlecriteria` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `criteriaId` bigint DEFAULT NULL,
  `criteriaDescription` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `criteriaAbbrDescription` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `criteriaType` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `criteriaUnitDescription` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `rawValue` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `valueKeyId` bigint DEFAULT NULL,
  `isMandatory` tinyint(1) DEFAULT '0',
  `isInterval` tinyint(1) DEFAULT '0',
  `successorCriteriaId` bigint DEFAULT NULL,
  `assemblyGroupNodeId` bigint DEFAULT NULL,
  `legacyArticleId` bigint DEFAULT NULL,
  `immediateDisplay` tinyint(1) DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=27074084 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Article criteria';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `articlecrosses`
--

DROP TABLE IF EXISTS `articlecrosses`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `articlecrosses` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `oemNumber` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `mfrId` bigint DEFAULT NULL,
  `brandName` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `assemblyGroupNodeId` bigint DEFAULT NULL,
  `legacyArticleId` bigint DEFAULT NULL,
  `number` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  KEY `artcleId` (`legacyArticleId`)
) ENGINE=InnoDB AUTO_INCREMENT=30370792 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Cross-references';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `articledocs`
--

DROP TABLE IF EXISTS `articledocs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `articledocs` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `docFileName` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `docFileTypeName` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `docId` bigint DEFAULT NULL,
  `docText` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `docTypeId` bigint DEFAULT NULL,
  `docUrl` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `assemblyGroupNodeId` bigint DEFAULT NULL,
  `legacyArticleId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=8203512 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Articles documents';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `articleean`
--

DROP TABLE IF EXISTS `articleean`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `articleean` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `legacyArticleId` bigint DEFAULT NULL,
  `eancode` varchar(25) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=4187238 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Spare parts EAN codes';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `articlelinks`
--

DROP TABLE IF EXISTS `articlelinks`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `articlelinks` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `url` mediumtext CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `legacyArticleId` bigint DEFAULT NULL,
  `lang` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `description` mediumtext CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  PRIMARY KEY (`id`) USING BTREE,
  KEY `legacyArticleId` (`legacyArticleId`) USING BTREE,
  KEY `lang` (`lang`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=317385 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Useful links to web resources';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `articlemain`
--

DROP TABLE IF EXISTS `articlemain`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `articlemain` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `mainArticleId` bigint DEFAULT NULL,
  `legacyArticleId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=2387976 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Main articles';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `articlepdfs`
--

DROP TABLE IF EXISTS `articlepdfs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `articlepdfs` (
  `id` int NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `url` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `fileName` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `typeDescription` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `headerDescription` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `assemblyGroupNodeId` int DEFAULT NULL COMMENT 'hide',
  `legacyArticleId` int DEFAULT NULL,
  `typeKeyId` int DEFAULT NULL COMMENT 'hide',
  `headerKeyId` int DEFAULT NULL COMMENT 'hide',
  `lang` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `articlePdfs_id_uindex` (`id`) USING BTREE,
  UNIQUE KEY `articlePdfs_la_u_uindex` (`legacyArticleId`,`url`) USING BTREE,
  KEY `articlePdfs_legacyArticleId_index` (`legacyArticleId`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=508354 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='PDF and other media';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `articles`
--

DROP TABLE IF EXISTS `articles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `articles` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `dataSupplierId` bigint DEFAULT NULL,
  `articleNumber` varchar(150) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  `mfrId` bigint NOT NULL,
  `additionalDescription` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `articleStatusId` bigint DEFAULT NULL COMMENT 'hide',
  `articleStatusDescription` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `articleStatusValidFromDate` bigint DEFAULT NULL,
  `quantityPerPackage` bigint DEFAULT NULL,
  `quantityPerPartPerPackage` bigint DEFAULT NULL,
  `isSelfServicePacking` tinyint(1) DEFAULT NULL,
  `hasMandatoryMaterialCertification` tinyint(1) DEFAULT NULL,
  `isRemanufacturedPart` tinyint(1) DEFAULT NULL,
  `isAccessory` tinyint(1) DEFAULT NULL,
  `genericArticleDescription` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `legacyArticleId` bigint unsigned DEFAULT NULL,
  `assemblyGroupNodeId` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  KEY `articleId_index` (`legacyArticleId`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=6893356 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Articles';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `articlesvehicletrees`
--

DROP TABLE IF EXISTS `articlesvehicletrees`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `articlesvehicletrees` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `legacyArticleId` bigint DEFAULT NULL COMMENT 'legacyArticleId',
  `assemblyGroupNodeId` bigint DEFAULT NULL,
  `linkingTargetId` bigint DEFAULT NULL,
  `linkingTargetType` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `legacyArticleId` (`legacyArticleId`,`assemblyGroupNodeId`,`linkingTargetId`,`linkingTargetType`) USING BTREE,
  KEY `legacyArticleId_3` (`legacyArticleId`) USING BTREE,
  KEY `assemblyGroupNodeId` (`assemblyGroupNodeId`,`linkingTargetId`,`linkingTargetType`) USING BTREE,
  KEY `linkingTargetId` (`linkingTargetId`,`linkingTargetType`) USING BTREE,
  KEY `linkingTargetId_2` (`linkingTargetId`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=651871453 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci ROW_FORMAT=DYNAMIC COMMENT='Links between vehicles and spare parts';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `articletext`
--

DROP TABLE IF EXISTS `articletext`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `articletext` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `infoId` bigint DEFAULT NULL,
  `informationTypeKey` bigint DEFAULT NULL,
  `informationTypeDescription` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `text` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `assemblyGroupNodeId` bigint DEFAULT NULL,
  `legacyArticleId` bigint DEFAULT NULL,
  `isImmediateDisplay` tinyint(1) DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=1607531 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Text information about spare parts';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `assemblygroupnodenames`
--

DROP TABLE IF EXISTS `assemblygroupnodenames`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `assemblygroupnodenames` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `assemblyGroupName` varchar(150) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `assemblyGroupNodeId` bigint DEFAULT NULL,
  `hasChilds` tinyint(1) DEFAULT NULL,
  `shortCutId` bigint DEFAULT NULL,
  `lang` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `parentNodeId` bigint DEFAULT '0',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `assemblyGroupName` (`assemblyGroupName`,`assemblyGroupNodeId`,`hasChilds`,`shortCutId`,`lang`,`parentNodeId`) USING BTREE,
  KEY `lang` (`lang`) USING BTREE,
  KEY `assemblyGroupNodesId_index` (`assemblyGroupNodeId`,`lang`) USING BTREE,
  KEY `lang_2` (`lang`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Assembly groups names';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `assemblygroupnodes`
--

DROP TABLE IF EXISTS `assemblygroupnodes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `assemblygroupnodes` (
  `id` int NOT NULL AUTO_INCREMENT,
  `assemblyGroupNodeId` bigint DEFAULT NULL,
  `hasChilds` tinyint(1) DEFAULT NULL,
  `shortCutId` bigint DEFAULT NULL,
  `parentNodeId` bigint DEFAULT '0',
  `linkingTargetId` bigint DEFAULT NULL,
  `linkingTargetType` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `assemblyGroupNodeId` (`assemblyGroupNodeId`,`hasChilds`,`shortCutId`,`parentNodeId`,`linkingTargetId`,`linkingTargetType`) USING BTREE,
  KEY `linkingTargetId` (`linkingTargetId`,`linkingTargetType`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci ROW_FORMAT=DYNAMIC COMMENT='Assembly groups';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `axlebodytype`
--

DROP TABLE IF EXISTS `axlebodytype`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `axlebodytype` (
  `bodyTypeName` varchar(150) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
  `axleId` bigint DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT COMMENT 'hide',
  PRIMARY KEY (`id`) USING BTREE,
  KEY `bodyTypeName` (`bodyTypeName`) USING BTREE,
  KEY `axleId` (`axleId`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=39406 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `axlebrakesizes`
--

DROP TABLE IF EXISTS `axlebrakesizes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `axlebrakesizes` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `brakeSize` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
  `brakeSizeId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `axleBrakeSizes_id_uindex` (`id`) USING BTREE,
  KEY `brakeSizeId` (`brakeSizeId`) USING BTREE,
  KEY `brakeSize` (`brakeSize`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=71 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `axledetails`
--

DROP TABLE IF EXISTS `axledetails`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `axledetails` (
  `id` int NOT NULL AUTO_INCREMENT,
  `axleBodyType` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `axleBodyTypeId` bigint DEFAULT NULL,
  `axleDescription` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `axleId` bigint DEFAULT NULL,
  `axleLoadFrom` bigint DEFAULT NULL,
  `axleLoadTo` bigint DEFAULT NULL,
  `axleManufacturer` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `axleModel` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `axleStyle` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `axleStyleId` bigint DEFAULT NULL,
  `axleType` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `axleTypeId` bigint DEFAULT NULL,
  `brakeType` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `brakeTypeId` bigint DEFAULT NULL,
  `wheelMount` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `wheelMountId` bigint DEFAULT NULL,
  `yearOfConstrFrom` bigint DEFAULT NULL,
  `yearOfConstrTo` bigint DEFAULT NULL,
  `driveHeightFrom` bigint DEFAULT NULL,
  `driveHeightTo` bigint DEFAULT NULL,
  `trackGauge` bigint DEFAULT NULL,
  `lang` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=7012 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Axle details';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `axles`
--

DROP TABLE IF EXISTS `axles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `axles` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `axleId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=7012 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Axles';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `bill`
--

DROP TABLE IF EXISTS `bill`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `bill` (
  `id` int NOT NULL AUTO_INCREMENT,
  `effective_date` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `payment_due_date` datetime DEFAULT NULL,
  `state` int NOT NULL DEFAULT '0',
  `discount` decimal(30,10) NOT NULL,
  `store_id` int NOT NULL,
  `sequence_number` int NOT NULL,
  `merchant_id` int NOT NULL,
  `maintenance_cost` decimal(30,10) NOT NULL,
  `note` text,
  `userName` varchar(45) DEFAULT NULL,
  `buyer_id` int DEFAULT NULL,
  `user_phone_number` varchar(10) DEFAULT NULL,
  `qr_code` varchar(1000) DEFAULT NULL,
  PRIMARY KEY (`id`),
  FULLTEXT KEY `note` (`note`,`userName`,`user_phone_number`)
) ENGINE=InnoDB AUTO_INCREMENT=277 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `bill_payment`
--

DROP TABLE IF EXISTS `bill_payment`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `bill_payment` (
  `id` int NOT NULL,
  `bill_id` int NOT NULL,
  `date` datetime NOT NULL,
  `amount` bigint NOT NULL,
  `currency_id` int NOT NULL,
  `pyament_method` int NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `bill_product`
--

DROP TABLE IF EXISTS `bill_product`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `bill_product` (
  `id` int NOT NULL AUTO_INCREMENT,
  `product_id` int DEFAULT NULL,
  `bill_id` int NOT NULL,
  `vat` decimal(5,2) DEFAULT '15.00',
  `price` decimal(12,2) NOT NULL,
  `quantity` decimal(10,3) NOT NULL,
  `total_before_vat` decimal(12,2) GENERATED ALWAYS AS (round((`price` * `quantity`),2)) STORED,
  `vat_total` decimal(12,2) GENERATED ALWAYS AS (round(((`total_before_vat` * `vat`) / 100),2)) STORED,
  `total_including_vat` decimal(12,2) GENERATED ALWAYS AS (round((`total_before_vat` + `vat_total`),2)) STORED,
  `name` varchar(255) DEFAULT NULL,
  `type` tinyint GENERATED ALWAYS AS ((case when (`product_id` is not null) then 0 when (`name` = _utf8mb4'maintenance_cost') then 2 else 1 end)) STORED NOT NULL,
  PRIMARY KEY (`id`),
  CONSTRAINT `chk_price` CHECK ((`price` > 0)),
  CONSTRAINT `chk_quantity` CHECK ((`quantity` > 0))
) ENGINE=InnoDB AUTO_INCREMENT=437 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Temporary view structure for view `bill_totals`
--

DROP TABLE IF EXISTS `bill_totals`;
/*!50001 DROP VIEW IF EXISTS `bill_totals`*/;
SET @saved_cs_client     = @@character_set_client;
/*!50503 SET character_set_client = utf8mb4 */;
/*!50001 CREATE VIEW `bill_totals` AS SELECT
 1 AS `id`,
 1 AS `effective_date`,
 1 AS `payment_due_date`,
 1 AS `state`,
 1 AS `discount`,
 1 AS `store_id`,
 1 AS `sequence_number`,
 1 AS `merchant_id`,
 1 AS `maintenance_cost`,
 1 AS `note`,
 1 AS `userName`,
 1 AS `buyer_id`,
 1 AS `user_phone_number`,
 1 AS `total_before_vat`,
 1 AS `total_vat`,
 1 AS `total`,
 1 AS `qr_code`*/;
SET character_set_client = @saved_cs_client;

--
-- Table structure for table `bodymark`
--

DROP TABLE IF EXISTS `bodymark`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `bodymark` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `linkedCars` bigint DEFAULT NULL COMMENT 'hide',
  `manuName` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `markId` bigint DEFAULT NULL,
  `markName` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `bodyMark_id_uindex` (`id`) USING BTREE,
  KEY `fk_bodyMark_bodyMarkCarIds_1` (`markId`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=50507 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `bodymarkcarids`
--

DROP TABLE IF EXISTS `bodymarkcarids`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `bodymarkcarids` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `carId` bigint DEFAULT NULL,
  `term` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `markId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `bodyMarkCarIds_id_uindex` (`id`) USING BTREE,
  KEY `fk_bodyMarkCarIds_cars_1` (`carId`) USING BTREE,
  KEY `fk_bodyMarkCarIds_bodyMark_1` (`markId`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=46397 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `car_link`
--

DROP TABLE IF EXISTS `car_link`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `car_link` (
  `linkageTargetId` bigint NOT NULL,
  `vehicleModelSeriesId` bigint NOT NULL,
  PRIMARY KEY (`linkageTargetId`,`vehicleModelSeriesId`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `cars`
--

DROP TABLE IF EXISTS `cars`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `cars` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `carId` bigint DEFAULT NULL,
  `carName` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `carType` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `firstCountry` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `manuId` bigint DEFAULT NULL,
  `modId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  KEY `fk_cars_manufacturers_1` (`manuId`) USING BTREE,
  KEY `fk_cars_modelSeries_1` (`modId`) USING BTREE,
  KEY `fk_cars_countries_1` (`firstCountry`) USING BTREE,
  KEY `carId` (`carId`) USING BTREE,
  KEY `carType` (`carType`) USING BTREE,
  KEY `carId_2` (`carId`,`carName`,`carType`,`firstCountry`,`manuId`,`modId`) USING BTREE,
  KEY `cars_manuId_index` (`manuId`) USING BTREE,
  KEY `cars_modId_index` (`modId`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=250299 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Vehicle types';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `cars_old`
--

DROP TABLE IF EXISTS `cars_old`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `cars_old` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `carId` bigint DEFAULT NULL,
  `carName` varchar(100) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci DEFAULT NULL,
  `carType` varchar(5) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci DEFAULT NULL,
  `firstCountry` varchar(5) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci DEFAULT NULL,
  `manuId` bigint DEFAULT NULL,
  `modId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  KEY `fk_cars_manufacturers_1` (`manuId`) USING BTREE,
  KEY `fk_cars_modelSeries_1` (`modId`) USING BTREE,
  KEY `fk_cars_countries_1` (`firstCountry`) USING BTREE,
  KEY `carId` (`carId`) USING BTREE,
  KEY `carType` (`carType`) USING BTREE,
  KEY `carId_2` (`carId`,`carName`,`carType`,`firstCountry`,`manuId`,`modId`) USING BTREE,
  KEY `cars_manuId_index` (`manuId`) USING BTREE,
  KEY `cars_modId_index` (`modId`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=223980 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Vehicle types OLD';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `carsbodies`
--

DROP TABLE IF EXISTS `carsbodies`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `carsbodies` (
  `manuId` bigint DEFAULT NULL,
  `modelId` bigint DEFAULT NULL,
  `carId` bigint DEFAULT NULL,
  `BodyId` bigint DEFAULT '0',
  `id` int NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `carType` varchar(1) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  KEY `BodyId` (`BodyId`) USING BTREE,
  KEY `manuId` (`manuId`,`BodyId`) USING BTREE,
  KEY `manuId_2` (`manuId`,`modelId`,`BodyId`) USING BTREE,
  KEY `manuId_3` (`manuId`,`modelId`,`carId`) USING BTREE,
  KEY `carId` (`carId`) USING BTREE,
  KEY `modelId` (`modelId`,`carType`) USING BTREE,
  KEY `carType` (`carType`) USING BTREE,
  KEY `manuId_4` (`manuId`,`BodyId`,`carType`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=91787 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `client`
--

DROP TABLE IF EXISTS `client`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `client` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `company_name` varchar(255) DEFAULT NULL,
  `email` varchar(255) DEFAULT NULL,
  `phone` varchar(20) DEFAULT NULL,
  `address` varchar(500) DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `vat_number` varchar(15) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_vat_number` (`vat_number`),
  UNIQUE KEY `uq_email` (`email`)
) ENGINE=InnoDB AUTO_INCREMENT=53 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `company`
--

DROP TABLE IF EXISTS `company`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `company` (
  `id` int NOT NULL AUTO_INCREMENT,
  `state` int NOT NULL DEFAULT '0',
  `name` varchar(45) NOT NULL,
  `vat_number` varchar(255) NOT NULL,
  `vat_registration_number` varchar(15) DEFAULT NULL,
  `commercial_registration_number` varchar(10) DEFAULT NULL,
  PRIMARY KEY (`id`),
  CONSTRAINT `chk_vat_registration_number` CHECK (regexp_like(`vat_registration_number`,_utf8mb4'^3[0-9]{13}3$'))
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `countries`
--

DROP TABLE IF EXISTS `countries`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `countries` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `countryCode` varchar(5) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci DEFAULT NULL,
  `countryName` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci,
  `usage` bigint DEFAULT NULL COMMENT 'hide',
  `lang` varchar(5) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `countries_id_uindex` (`id`) USING BTREE,
  KEY `countryCode` (`countryCode`) USING BTREE,
  KEY `lang` (`lang`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=9946 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Countries';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `countrygroups`
--

DROP TABLE IF EXISTS `countrygroups`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `countrygroups` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `countryName` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `tecdocCode` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `lang` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `countryGroups_id_uindex` (`id`) USING BTREE,
  KEY `fk_countryGroups_countries_1` (`tecdocCode`) USING BTREE,
  KEY `fk_countryGroups_countries_2` (`lang`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=937 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Country groups';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `credit_note`
--

DROP TABLE IF EXISTS `credit_note`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `credit_note` (
  `id` int NOT NULL AUTO_INCREMENT,
  `bill_id` int DEFAULT NULL,
  `state` int DEFAULT NULL,
  `NOTE` text,
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `unique_bill_id` (`bill_id`),
  CONSTRAINT `credit_note_ibfk_1` FOREIGN KEY (`bill_id`) REFERENCES `bill` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=78 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `criteria`
--

DROP TABLE IF EXISTS `criteria`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `criteria` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `criteriaId` bigint DEFAULT NULL,
  `criteriaName` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `criteriaShortName` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `criteriaType` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `isInterval` tinyint(1) DEFAULT NULL,
  `lang` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `criteriaUnit` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `successorId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='List of all criterias';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `genericarticles`
--

DROP TABLE IF EXISTS `genericarticles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `genericarticles` (
  `articleId` bigint DEFAULT NULL,
  `genericArticleId` bigint DEFAULT NULL,
  KEY `artcleIdIndex` (`articleId`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Generic articles';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `genericarticlesgroups`
--

DROP TABLE IF EXISTS `genericarticlesgroups`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `genericarticlesgroups` (
  `assemblyGroup` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `designation` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `genericArticleId` bigint DEFAULT NULL,
  `masterDesignation` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `lang` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `usageDesignation` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Generic articles groups';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `keyvalues`
--

DROP TABLE IF EXISTS `keyvalues`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `keyvalues` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `keyId` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `keyTableId` bigint DEFAULT NULL,
  `keyValue` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `lang` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='All values for criterias';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `languages`
--

DROP TABLE IF EXISTS `languages`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `languages` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `languageCode` varchar(5) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci DEFAULT NULL,
  `languageName` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci,
  `lang` varchar(5) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `languages_id_uindex` (`id`) USING BTREE,
  KEY `lang` (`lang`) USING BTREE,
  KEY `languageCode` (`languageCode`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=1522 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Languages';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `legacy2generic`
--

DROP TABLE IF EXISTS `legacy2generic`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `legacy2generic` (
  `legacyArticleId` bigint unsigned NOT NULL,
  `genericArticleId` bigint DEFAULT NULL,
  `id` int NOT NULL AUTO_INCREMENT COMMENT 'hide',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `legacyArticleId_2` (`legacyArticleId`,`genericArticleId`),
  KEY `legacyArticleId` (`legacyArticleId`) USING BTREE,
  KEY `genericArticleId` (`genericArticleId`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=6196251 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `linkagetargets`
--

DROP TABLE IF EXISTS `linkagetargets`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `linkagetargets` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `linkageTargetId` bigint NOT NULL,
  `linkageTargetType` varchar(15) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `description` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `mfrId` bigint NOT NULL,
  `mfrName` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `mfrShortName` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `beginYearMonth` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `endYearMonth` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `lang` varchar(3) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `subLinkageTargetType` varchar(15) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `vehicleModelSeriesId` bigint NOT NULL,
  `vehicleModelSeriesName` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `rmiTypeId` bigint NOT NULL,
  `imageURL50` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `imageURL100` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `imageURL200` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `imageURL400` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `imageURL800` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `0` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `fuelMixtureFormationTypeKey` bigint NOT NULL,
  `fuelMixtureFormationType` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `driveTypeKey` bigint NOT NULL,
  `driveType` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `bodyStyleKey` bigint NOT NULL,
  `bodyStyle` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `valves` bigint NOT NULL,
  `fuelTypeKey` bigint NOT NULL,
  `fuelType` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `engineTypeKey` bigint NOT NULL,
  `engineType` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `horsePowerFrom` bigint NOT NULL,
  `horsePowerTo` bigint NOT NULL,
  `kiloWattsFrom` bigint NOT NULL,
  `kiloWattsTo` bigint NOT NULL,
  `cylinders` bigint NOT NULL,
  `capacityCC` bigint NOT NULL,
  `capacityLiters` double NOT NULL,
  `code` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `axleStyleKey` bigint NOT NULL,
  `axleStyle` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `axleTypeKey` bigint NOT NULL,
  `axleType` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `axleBodyKey` bigint NOT NULL,
  `axleBody` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `wheelMountingKey` bigint NOT NULL,
  `wheelMounting` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `axleLoadToKg` bigint NOT NULL,
  `brakeTypeKey` bigint NOT NULL,
  `brakeType` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `hmdMfrModelId` bigint NOT NULL,
  `hmdMfrModelName` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `aspirationKey` bigint NOT NULL,
  `aspiration` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `cylinderDesignKey` bigint NOT NULL,
  `cylinderDesign` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `coolingTypeKey` bigint NOT NULL,
  `coolingType` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `tonnage` bigint NOT NULL,
  `axleConfigurationKey` bigint NOT NULL,
  `axleConfiguration` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci NOT NULL,
  `axleLoadFromKg` bigint NOT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  KEY `linkageTargetId_2` (`linkageTargetId`,`linkageTargetType`,`lang`) USING BTREE,
  KEY `idx_model_en` (`vehicleModelSeriesId`,`lang`),
  KEY `idx_vehicleModelSeriesId` (`vehicleModelSeriesId`),
  KEY `base_idx2` (`linkageTargetId`,`vehicleModelSeriesId`),
  KEY `linkageTargetIdx` (`linkageTargetId`),
  KEY `vehicleModelSeriesIdx` (`vehicleModelSeriesId`)
) ENGINE=InnoDB AUTO_INCREMENT=943318 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Detailed information about vehicles';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `manufacturermotorids`
--

DROP TABLE IF EXISTS `manufacturermotorids`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `manufacturermotorids` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `motorId` bigint DEFAULT NULL,
  `manuId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=33313 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Motor manufacturer IDs';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `manufacturers`
--

DROP TABLE IF EXISTS `manufacturers`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `manufacturers` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `manuId` bigint DEFAULT NULL,
  `manuName` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `linkingTargetType` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `manufacturers_id_uindex` (`id`) USING BTREE,
  KEY `manuId` (`manuId`) USING BTREE,
  KEY `linkingTargetType` (`linkingTargetType`) USING BTREE,
  FULLTEXT KEY `manuName` (`manuName`)
) ENGINE=InnoDB AUTO_INCREMENT=3797 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Description of all manufacturers';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `modelseries`
--

DROP TABLE IF EXISTS `modelseries`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `modelseries` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `modelId` bigint DEFAULT NULL,
  `modelname` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `yearOfConstrTo` bigint DEFAULT NULL,
  `yearOfConstrFrom` bigint DEFAULT NULL,
  `linkingTargetType` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `manuId` bigint DEFAULT NULL,
  `start_year` int DEFAULT NULL,
  `end_year` int DEFAULT NULL,
  `model_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `modelSeries_id_uindex` (`id`) USING BTREE,
  UNIQUE KEY `modelSeries_modelId_linkingTargetType_manuId_uindex` (`modelId`,`linkingTargetType`,`manuId`) USING BTREE,
  KEY `modelSeries_modelId_index` (`modelId`) USING BTREE,
  KEY `modelSeries_manuId_index` (`manuId`) USING BTREE,
  KEY `modelSeries_linkingTargetType_index` (`linkingTargetType`) USING BTREE,
  KEY `idx_all` (`manuId`,`modelname`,`yearOfConstrTo`,`yearOfConstrFrom`),
  KEY `base_idx` (`start_year`,`end_year`,`manuId`),
  KEY `base_idx2` (`modelname`,`manuId`,`start_year`,`end_year`),
  FULLTEXT KEY `modelname` (`modelname`),
  FULLTEXT KEY `model_name` (`model_name`)
) ENGINE=InnoDB AUTO_INCREMENT=34125 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Vehicle models - Linking target type:\r\nP: Passenger car\r\nO: Commercial vehicle\r\nM: Motor\r\nA: Axles\r\nK: Body type';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `modelseries_old`
--

DROP TABLE IF EXISTS `modelseries_old`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `modelseries_old` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `modelId` bigint DEFAULT NULL,
  `modelname` varchar(255) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci DEFAULT NULL,
  `yearOfConstrTo` bigint DEFAULT NULL,
  `yearOfConstrFrom` bigint DEFAULT NULL,
  `linkingTargetType` varchar(5) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci DEFAULT NULL,
  `manuId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `modelSeries_id_uindex` (`id`) USING BTREE,
  UNIQUE KEY `modelSeries_modelId_linkingTargetType_manuId_uindex` (`modelId`,`linkingTargetType`,`manuId`) USING BTREE,
  KEY `modelSeries_modelId_index` (`modelId`) USING BTREE,
  KEY `modelSeries_manuId_index` (`manuId`) USING BTREE,
  KEY `modelSeries_linkingTargetType_index` (`linkingTargetType`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=26025 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Vehicle models - Linking target type:\r\nP: Passenger car\r\nO: Commercial vehicle\r\nM: Motor\r\nA: Axles\r\nK: Body type';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `motordetails`
--

DROP TABLE IF EXISTS `motordetails`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `motordetails` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `motorId` bigint DEFAULT NULL,
  `boreDiameter` bigint DEFAULT NULL,
  `charging` varchar(150) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `compressionFrom` bigint DEFAULT NULL,
  `constructionType` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `control` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `cooling` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `crankshafts` bigint DEFAULT NULL,
  `cylinder` bigint DEFAULT NULL,
  `cylinderCapacity` bigint DEFAULT NULL,
  `cylinderDesign` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `fuelPreperation` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `fuelType` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `litersTechFrom` bigint DEFAULT NULL,
  `manuId` bigint DEFAULT NULL,
  `manuText` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `motorCode` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `motorNumber` bigint DEFAULT NULL,
  `motorType` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `powerHP` bigint DEFAULT NULL,
  `powerKW` bigint DEFAULT NULL,
  `travel` bigint DEFAULT NULL,
  `valveControl` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `valves` bigint DEFAULT NULL,
  `lang` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `emissionStandard` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `rpmKwFrom` bigint DEFAULT NULL,
  `rpmTorqueFrom` bigint DEFAULT NULL,
  `sellsTerm` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `torqueFrom` bigint DEFAULT NULL,
  `ccmTaxFrom` bigint DEFAULT NULL,
  `powerHpTo` bigint DEFAULT NULL,
  `powerKwTo` bigint DEFAULT NULL,
  `rpmTorqueTo` bigint DEFAULT NULL,
  `rpmKwTo` bigint DEFAULT NULL,
  `litersTaxFrom` bigint DEFAULT NULL,
  `yearOfConstrFrom` bigint DEFAULT NULL,
  `compressionTo` bigint DEFAULT NULL,
  `torqueTo` bigint DEFAULT NULL,
  `yearOfConstrTo` bigint DEFAULT NULL,
  `litersTechTo` bigint DEFAULT NULL,
  `ccmTaxTo` bigint DEFAULT NULL,
  `litersTaxTo` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=33313 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Motor details';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `newarticles`
--

DROP TABLE IF EXISTS `newarticles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `newarticles` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `articleId` bigint DEFAULT NULL,
  `articleNumber` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Unused info';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `notification_settings`
--

DROP TABLE IF EXISTS `notification_settings`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `notification_settings` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL,
  `low_stock_alert` tinyint(1) NOT NULL DEFAULT '1',
  `low_stock_threshold` int unsigned NOT NULL DEFAULT '5',
  `pending_invoice_days` int unsigned NOT NULL DEFAULT '7',
  `new_order_alert` tinyint(1) NOT NULL DEFAULT '1',
  `paymen_due_alert` tinyint(1) NOT NULL DEFAULT '1',
  `daily_summary` tinyint(1) NOT NULL DEFAULT '0',
  `email_enabled` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_notif_settings_user` (`user_id`),
  CONSTRAINT `fk_notif_settings_user` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `notifications`
--

DROP TABLE IF EXISTS `notifications`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `notifications` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL,
  `type` tinyint NOT NULL DEFAULT '0',
  `title` varchar(255) NOT NULL,
  `message` text NOT NULL,
  `is_read` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `fk_notif_user` (`user_id`),
  CONSTRAINT `fk_notif_user` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `oem_number`
--

DROP TABLE IF EXISTS `oem_number`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `oem_number` (
  `id` int NOT NULL AUTO_INCREMENT,
  `number` varchar(255) NOT NULL,
  `articleId` bigint DEFAULT NULL,
  `clean_number` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `id_UNIQUE` (`id`) /*!80000 INVISIBLE */,
  UNIQUE KEY `number_UNIQUE` (`number`) /*!80000 INVISIBLE */,
  KEY `idx_article_id` (`articleId`),
  KEY `base_idx` (`number`,`articleId`),
  FULLTEXT KEY `full_text_search` (`number`),
  FULLTEXT KEY `clean_number` (`clean_number`)
) ENGINE=InnoDB AUTO_INCREMENT=21550894 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `oemnumbers`
--

DROP TABLE IF EXISTS `oemnumbers`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `oemnumbers` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `articleNumber` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `mfrId` bigint DEFAULT NULL,
  `assemblyGroupNodeId` bigint DEFAULT NULL,
  `legacyArticleId` bigint DEFAULT NULL,
  `lang` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `referenceTypeKey` varchar(25) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `referenceTypeDescription` varchar(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `oemNumbers_id_uindex` (`id`) USING BTREE,
  UNIQUE KEY `articleNumber` (`articleNumber`,`mfrId`,`assemblyGroupNodeId`,`legacyArticleId`,`lang`,`referenceTypeKey`,`referenceTypeDescription`) USING BTREE,
  KEY `fk_oemNumbers_articles_1` (`legacyArticleId`) USING BTREE,
  KEY `fk_oemNumbers_articles_3` (`articleNumber`) USING BTREE,
  KEY `fk_oemNumbers_articles_4` (`mfrId`) USING BTREE,
  KEY `fk_oemNumbers_articles_6` (`assemblyGroupNodeId`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=1912176 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='OE article numbers';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `product`
--

DROP TABLE IF EXISTS `product`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `product` (
  `id` int NOT NULL AUTO_INCREMENT,
  `article_id` int NOT NULL,
  `store_id` int NOT NULL,
  `status` int NOT NULL DEFAULT '0',
  `shelf_number` varchar(45) DEFAULT NULL,
  `min_stock` int NOT NULL DEFAULT '5',
  `cost_price` decimal(12,2) NOT NULL DEFAULT '0.00',
  `price` decimal(12,2) NOT NULL,
  `quantity` decimal(10,3) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `id_UNIQUE` (`article_id`,`store_id`) /*!80000 INVISIBLE */,
  CONSTRAINT `ch_product_price` CHECK ((`price` > 0)),
  CONSTRAINT `ch_product_quantity` CHECK ((`quantity` >= 0))
) ENGINE=InnoDB AUTO_INCREMENT=159 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `purchase_bill`
--

DROP TABLE IF EXISTS `purchase_bill`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `purchase_bill` (
  `id` int NOT NULL AUTO_INCREMENT,
  `effective_date` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `payment_due_date` datetime DEFAULT NULL,
  `state` int NOT NULL DEFAULT '0',
  `discount` bigint NOT NULL DEFAULT '0',
  `supplier_id` int NOT NULL,
  `sequence_number` int NOT NULL,
  `supplier_sequence_number` int DEFAULT NULL,
  `vat_sequence_number` int DEFAULT NULL,
  `store_id` int NOT NULL,
  `merchant_id` int NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=126 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `purchase_bill_payment`
--

DROP TABLE IF EXISTS `purchase_bill_payment`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `purchase_bill_payment` (
  `id` int NOT NULL AUTO_INCREMENT,
  `purchase_register_id` int NOT NULL,
  `date` datetime DEFAULT NULL,
  `amount` decimal(10,0) NOT NULL,
  `currency_id` int DEFAULT NULL,
  `payment_method` int DEFAULT NULL,
  `product_id` int NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `purchase_bill_product`
--

DROP TABLE IF EXISTS `purchase_bill_product`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `purchase_bill_product` (
  `id` int NOT NULL AUTO_INCREMENT,
  `product_id` int DEFAULT NULL,
  `bill_id` int NOT NULL,
  `vat` decimal(5,2) DEFAULT '15.00',
  `price` decimal(12,2) NOT NULL,
  `quantity` decimal(10,3) NOT NULL,
  `total_before_vat` decimal(12,2) GENERATED ALWAYS AS (round((`price` * `quantity`),2)) STORED,
  `vat_total` decimal(12,2) GENERATED ALWAYS AS (round(((`total_before_vat` * `vat`) / 100),2)) STORED,
  `total_including_vat` decimal(12,2) GENERATED ALWAYS AS (round((`total_before_vat` + `vat_total`),2)) STORED,
  `name` varchar(255) DEFAULT NULL,
  `type` tinyint GENERATED ALWAYS AS ((case when (`product_id` is not null) then 0 else 1 end)) STORED NOT NULL,
  PRIMARY KEY (`id`),
  CONSTRAINT `chpk_price` CHECK ((`price` > 0)),
  CONSTRAINT `chpk_quantity` CHECK ((`quantity` > 0))
) ENGINE=InnoDB AUTO_INCREMENT=179 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Temporary view structure for view `purchase_bill_totals`
--

DROP TABLE IF EXISTS `purchase_bill_totals`;
/*!50001 DROP VIEW IF EXISTS `purchase_bill_totals`*/;
SET @saved_cs_client     = @@character_set_client;
/*!50503 SET character_set_client = utf8mb4 */;
/*!50001 CREATE VIEW `purchase_bill_totals` AS SELECT
 1 AS `id`,
 1 AS `effective_date`,
 1 AS `payment_due_date`,
 1 AS `state`,
 1 AS `discount`,
 1 AS `supplier_id`,
 1 AS `sequence_number`,
 1 AS `supplier_sequence_number`,
 1 AS `vat_sequence_number`,
 1 AS `store_id`,
 1 AS `merchant_id`,
 1 AS `total_before_vat`,
 1 AS `total_vat`,
 1 AS `total`*/;
SET character_set_client = @saved_cs_client;

--
-- Table structure for table `refresh_token`
--

DROP TABLE IF EXISTS `refresh_token`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `refresh_token` (
  `id` char(36) NOT NULL DEFAULT (uuid()),
  `user_id` int NOT NULL,
  `token_hash` varchar(64) NOT NULL,
  `device_name` varchar(100) DEFAULT NULL,
  `ip_address` varchar(45) DEFAULT NULL,
  `revoked` tinyint(1) NOT NULL DEFAULT '0',
  `expires_at` datetime NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_rt_user` (`user_id`),
  KEY `idx_rt_hash` (`token_hash`),
  KEY `idx_rt_expires` (`expires_at`),
  CONSTRAINT `fk_rt_user` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `replacedbyarticles`
--

DROP TABLE IF EXISTS `replacedbyarticles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `replacedbyarticles` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `articleNumber` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `dataSupplierId` bigint DEFAULT NULL,
  `mfrId` bigint DEFAULT NULL,
  `assemblyGroupNodeId` bigint DEFAULT NULL,
  `legacyArticleId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=229424 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Replaced by articles';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `replacesarticles`
--

DROP TABLE IF EXISTS `replacesarticles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `replacesarticles` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `articleNumber` varchar(250) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `dataSupplierId` bigint DEFAULT NULL,
  `mfrId` bigint DEFAULT NULL,
  `assemblyGroupNodeId` bigint DEFAULT NULL,
  `legacyArticleId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=240835 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Replaces articles';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `searchindex`
--

DROP TABLE IF EXISTS `searchindex`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `searchindex` (
  `id` int NOT NULL AUTO_INCREMENT,
  `legacyArticleId` int unsigned DEFAULT NULL,
  `keywords` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `legacyArticleId` (`legacyArticleId`) USING BTREE,
  FULLTEXT KEY `keywords` (`keywords`)
) ENGINE=MyISAM AUTO_INCREMENT=5806070 DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Search index';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `shortcuts`
--

DROP TABLE IF EXISTS `shortcuts`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `shortcuts` (
  `shortCutId` bigint DEFAULT NULL,
  `shortCutName` text CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci,
  `linkingTargetType` varchar(5) CHARACTER SET utf8mb3 COLLATE utf8mb3_unicode_ci DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Shortcuts to vehicles main parts';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `store`
--

DROP TABLE IF EXISTS `store`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `store` (
  `id` int NOT NULL AUTO_INCREMENT,
  `addressId` int DEFAULT NULL,
  `status` int NOT NULL DEFAULT '0',
  `company_id` int NOT NULL,
  `name` varchar(255) DEFAULT NULL,
  `address_name` text,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `companyID_idx` (`company_id`),
  CONSTRAINT `companyID` FOREIGN KEY (`company_id`) REFERENCES `company` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `supplier`
--

DROP TABLE IF EXISTS `supplier`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `supplier` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `company_id` int NOT NULL,
  `name` varchar(255) DEFAULT NULL,
  `address` varchar(255) DEFAULT NULL,
  `phone_number` varchar(255) DEFAULT NULL,
  `number` varchar(255) DEFAULT NULL,
  `vat_number` varchar(255) DEFAULT NULL,
  `is_deleted` tinyint(1) DEFAULT '0',
  `bank_account` varchar(255) DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `company_id` (`company_id`),
  CONSTRAINT `supplier_ibfk_1` FOREIGN KEY (`company_id`) REFERENCES `company` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=92 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `tradenumbers`
--

DROP TABLE IF EXISTS `tradenumbers`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tradenumbers` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `tradeNumber` varchar(150) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `legacyArticleId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=1913184 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Spare Parts Trade Numbers';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `user`
--

DROP TABLE IF EXISTS `user`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `user` (
  `id` int NOT NULL AUTO_INCREMENT,
  `username` varchar(45) NOT NULL,
  `password` varchar(100) NOT NULL,
  `company_id` int DEFAULT NULL,
  `email` varchar(255) DEFAULT NULL,
  `phone` varchar(20) DEFAULT NULL,
  `is_active` tinyint(1) NOT NULL DEFAULT '1',
  `last_login` datetime DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=12 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `vehicleaxles`
--

DROP TABLE IF EXISTS `vehicleaxles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `vehicleaxles` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `axleDescription` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `axleManufacturer` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `axleModel` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `axlePosition` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `lang` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `carId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=55041 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Axles descriptions';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `vehicledetails`
--

DROP TABLE IF EXISTS `vehicledetails`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `vehicledetails` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `carId` bigint DEFAULT NULL,
  `ccmTech` bigint DEFAULT NULL,
  `constructionType` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `cylinder` bigint DEFAULT NULL,
  `cylinderCapacityCcm` bigint DEFAULT NULL,
  `cylinderCapacityLiter` bigint DEFAULT NULL,
  `fuelType` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `fuelTypeProcess` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `impulsionType` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `manuId` bigint DEFAULT NULL,
  `manuName` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `modId` bigint DEFAULT NULL,
  `modelName` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `motorType` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `powerHpFrom` bigint DEFAULT NULL,
  `powerHpTo` bigint DEFAULT NULL,
  `powerKwFrom` bigint DEFAULT NULL,
  `powerKwTo` bigint DEFAULT NULL,
  `typeName` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `typeNumber` bigint DEFAULT NULL,
  `valves` bigint DEFAULT NULL,
  `yearOfConstrFrom` bigint DEFAULT NULL,
  `yearOfConstrTo` bigint DEFAULT NULL,
  `lang` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `axisConfiguration` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `tonnage` bigint DEFAULT NULL,
  `brakeSystem` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=64335 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Additional information about vehicle type';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `vehiclemotorcodes`
--

DROP TABLE IF EXISTS `vehiclemotorcodes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `vehiclemotorcodes` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `carId` bigint NOT NULL,
  `motorCode` varchar(150) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=76269 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Vehicle motor codes';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `vehicleprototypes`
--

DROP TABLE IF EXISTS `vehicleprototypes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `vehicleprototypes` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `carId` bigint DEFAULT NULL,
  `prototype` varchar(250) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=54616 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Vehicles prototypes';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `vehiclesecondarytypes`
--

DROP TABLE IF EXISTS `vehiclesecondarytypes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `vehiclesecondarytypes` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `subTypeDescription` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
  `subTypeId` bigint DEFAULT NULL,
  `carId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=2241 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Vehicle secondary types';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `vehicletrees`
--

DROP TABLE IF EXISTS `vehicletrees`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `vehicletrees` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'hide',
  `assemblyGroupNodeId` bigint DEFAULT NULL,
  `parentNodeId` bigint DEFAULT NULL,
  `sortNo` bigint DEFAULT NULL,
  `carId` bigint DEFAULT NULL,
  `linkingTargetType` varchar(5) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `vehicleTrees_id_uindex` (`id`) USING BTREE,
  UNIQUE KEY `assemblyGroupNodeId_2` (`assemblyGroupNodeId`,`parentNodeId`,`sortNo`,`carId`,`linkingTargetType`) USING BTREE,
  KEY `parentNodeId` (`parentNodeId`) USING BTREE,
  KEY `sortNo` (`sortNo`) USING BTREE,
  KEY `carId` (`carId`) USING BTREE,
  KEY `linkingTargetType` (`linkingTargetType`) USING BTREE,
  KEY `assemblyGroupNodeId` (`assemblyGroupNodeId`,`parentNodeId`,`carId`,`linkingTargetType`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=25399489 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Vehicle parts search trees';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `vehiclewheelbases`
--

DROP TABLE IF EXISTS `vehiclewheelbases`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `vehiclewheelbases` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `axlePosition` varchar(150) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `wheelbase` bigint DEFAULT NULL,
  `wheelbaseId` bigint DEFAULT NULL,
  `carId` bigint DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=99365 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='Vehicle wheel bases';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `vin_cache`
--

DROP TABLE IF EXISTS `vin_cache`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `vin_cache` (
  `id` int NOT NULL AUTO_INCREMENT,
  `vin` varchar(20) NOT NULL,
  `data` json NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `number_UNIQUE` (`vin`)
) ENGINE=InnoDB AUTO_INCREMENT=45 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Final view structure for view `bill_totals`
--

/*!50001 DROP VIEW IF EXISTS `bill_totals`*/;
/*!50001 SET @saved_cs_client          = @@character_set_client */;
/*!50001 SET @saved_cs_results         = @@character_set_results */;
/*!50001 SET @saved_col_connection     = @@collation_connection */;
/*!50001 SET character_set_client      = utf8mb4 */;
/*!50001 SET character_set_results     = utf8mb4 */;
/*!50001 SET collation_connection      = utf8mb4_0900_ai_ci */;
/*!50001 CREATE ALGORITHM=UNDEFINED */
/*!50013 DEFINER=`root`@`localhost` SQL SECURITY DEFINER */
/*!50001 VIEW `bill_totals` AS select `b`.`id` AS `id`,`b`.`effective_date` AS `effective_date`,`b`.`payment_due_date` AS `payment_due_date`,`b`.`state` AS `state`,`b`.`discount` AS `discount`,`b`.`store_id` AS `store_id`,`b`.`sequence_number` AS `sequence_number`,`b`.`merchant_id` AS `merchant_id`,`b`.`maintenance_cost` AS `maintenance_cost`,`b`.`note` AS `note`,`b`.`userName` AS `userName`,`b`.`buyer_id` AS `buyer_id`,`b`.`user_phone_number` AS `user_phone_number`,round(coalesce(sum(`bp`.`total_before_vat`),0),2) AS `total_before_vat`,round(coalesce(sum(`bp`.`vat_total`),0),2) AS `total_vat`,round(coalesce(sum(`bp`.`total_including_vat`),0),2) AS `total`,`b`.`qr_code` AS `qr_code` from (`bill` `b` left join `bill_product` `bp` on((`b`.`id` = `bp`.`bill_id`))) group by `b`.`id` */;
/*!50001 SET character_set_client      = @saved_cs_client */;
/*!50001 SET character_set_results     = @saved_cs_results */;
/*!50001 SET collation_connection      = @saved_col_connection */;

--
-- Final view structure for view `purchase_bill_totals`
--

/*!50001 DROP VIEW IF EXISTS `purchase_bill_totals`*/;
/*!50001 SET @saved_cs_client          = @@character_set_client */;
/*!50001 SET @saved_cs_results         = @@character_set_results */;
/*!50001 SET @saved_col_connection     = @@collation_connection */;
/*!50001 SET character_set_client      = utf8mb4 */;
/*!50001 SET character_set_results     = utf8mb4 */;
/*!50001 SET collation_connection      = utf8mb4_0900_ai_ci */;
/*!50001 CREATE ALGORITHM=UNDEFINED */
/*!50013 DEFINER=`root`@`localhost` SQL SECURITY DEFINER */
/*!50001 VIEW `purchase_bill_totals` AS select `b`.`id` AS `id`,`b`.`effective_date` AS `effective_date`,`b`.`payment_due_date` AS `payment_due_date`,`b`.`state` AS `state`,`b`.`discount` AS `discount`,`b`.`supplier_id` AS `supplier_id`,`b`.`sequence_number` AS `sequence_number`,`b`.`supplier_sequence_number` AS `supplier_sequence_number`,`b`.`vat_sequence_number` AS `vat_sequence_number`,`b`.`store_id` AS `store_id`,`b`.`merchant_id` AS `merchant_id`,round(coalesce(sum(`bp`.`total_before_vat`),0),2) AS `total_before_vat`,round(coalesce(sum(`bp`.`vat_total`),0),2) AS `total_vat`,round(coalesce(sum(`bp`.`total_including_vat`),0),2) AS `total` from (`purchase_bill` `b` left join `purchase_bill_product` `bp` on((`b`.`id` = `bp`.`bill_id`))) group by `b`.`id` */;
/*!50001 SET character_set_client      = @saved_cs_client */;
/*!50001 SET character_set_results     = @saved_cs_results */;
/*!50001 SET collation_connection      = @saved_col_connection */;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2026-03-06 21:45:29
