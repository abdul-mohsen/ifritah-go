-- name: Register :execresult
INSERT INTO user (company_id, username, password ) VALUES (?, ?, ?);

-- login
SELECT id, password FROM user where username = ? limit 1;
