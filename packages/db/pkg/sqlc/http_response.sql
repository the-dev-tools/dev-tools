-- name: GetHTTPResponseHeadersByHttpID :many
SELECT hrh.id, hrh.response_id, hrh.key, hrh.value, hrh.created_at
FROM http_response_header hrh
JOIN http_response hr ON hrh.response_id = hr.id
WHERE hr.http_id = ?
ORDER BY hrh.created_at DESC;

-- name: GetHTTPResponseAssertsByHttpID :many
SELECT hra.id, hra.response_id, hra.value, hra.success, hra.created_at
FROM http_response_assert hra
JOIN http_response hr ON hra.response_id = hr.id
WHERE hr.http_id = ?
ORDER BY hra.created_at DESC;
