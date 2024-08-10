package model

type ArticleTable struct {
  ID int64
  DataSupplierId int64
  articleNumber string
  MfrId int64
  AdditionalDescription string
  ArticleStatusId int64
  ArticleStatusDescription string
  ArticleStatusValidFromDate int64
  QuantityPerPackage int64
  QuantityPerPartPerPackage int64
  IsSelfServicePacking bool
  HasMandatoryMaterialCertification bool
  IsRemanufacturedPart bool
  IsAccessory bool
  GenericArticleDescription string
  LegacyArticleId int64
  AssemblyGroupNodeId int64
}
