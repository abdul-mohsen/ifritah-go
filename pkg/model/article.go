package model

import "database/sql"

type ArticleTable struct {
  ID int64
  DataSupplierId int64
  ArticleNumber string
  MfrId int64
  AdditionalDescription string
  ArticleStatusId int64
  ArticleStatusDescription string
  ArticleStatusValidFromDate int64
  QuantityPerPackage int64
  QuantityPerPartPerPackage int64
  IsSelfServicePacking sql.NullBool
  HasMandatoryMaterialCertification sql.NullBool
  IsRemanufacturedPart sql.NullBool
  IsAccessory sql.NullBool
  GenericArticleDescription string
  LegacyArticleId int64
  AssemblyGroupNodeId int64
}
