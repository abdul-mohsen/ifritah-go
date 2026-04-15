-- name: GetBranchByBillID :one
SELECT branches.id as branch_id from bill join branches on branch_id = branches.id where is_deleted = FALSE and bill.id = ? limit 1;


-- name: GetZatcaBranchConfig :one
SELECT
COALESCE(csr_org_identifier,'') as orgID, COALESCE(csr_org_unit,'') as orgUnit,
COALESCE(csr_org_name,'') as orgName, COALESCE(csr_country,'SA') as csrCountry,
COALESCE(csr_location,'') as csrLoc, COALESCE(business_category,'Supply activities') as bizCat,
COALESCE(seller_vat,'') as vat, COALESCE(seller_crn,'') as crn,
COALESCE(street,'') as street, COALESCE(building,'') as building, COALESCE(district,'') as district,
COALESCE(postal_code,'') as postal,
LENGTH(COALESCE(zatca_csr,'')) as csrLen,
LENGTH(COALESCE(zatca_production_username,'')) as prodLen,
zatca_registered_at,
zatca_status,
zatca_onboarded_at,
b.name
FROM branch_zatca_config as config join branches as b on b.id = config.branch_id WHERE branch_id = ? limit 1;

-- name: GetBranchName :one
SELECT name FROM branches WHERE id = ?;

-- name: UpdateZatcaBranchConfig :exec
INSERT INTO branch_zatca_config
(branch_id, csr_org_identifier, csr_org_unit, csr_org_name,
  csr_country, csr_location, business_category,
  seller_vat, seller_crn, street, building, district, postal_code, zatca_status)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
csr_org_identifier = VALUES(csr_org_identifier),
csr_org_unit = VALUES(csr_org_unit),
csr_org_name = VALUES(csr_org_name),
csr_country = VALUES(csr_country),
csr_location = VALUES(csr_location),
business_category = VALUES(business_category),
seller_vat = VALUES(seller_vat),
seller_crn = VALUES(seller_crn),
street = VALUES(street),
building = VALUES(building),
district = VALUES(district),
postal_code = VALUES(postal_code),
zatca_status = VALUES(zatca_status);
