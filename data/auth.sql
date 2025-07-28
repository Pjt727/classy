-- name: AuthInsertUser :exec
INSERT INTO management_users (username, encrypted_password)
VALUES (@username, @encrypted_password);

-- name: AuthGetUser :one
SELECT * FROM management_users
WHERE username = @username;

