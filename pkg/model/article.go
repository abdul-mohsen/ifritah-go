package model

type ArticleTable struct {
  ID int32
  DataSupplierId int64
  articleNumber string
  mfrId int64
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
  LegacyArticleId int32
  AssemblyGroupNodeId int64
}
