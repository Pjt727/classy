-- name: GetPreviousCollections :exec
SELECT * FROM schools;

-- name: GetTermCollection :one
SELECT * FROM  term_collections
WHERE term_collections.id = @id
      AND term_collections.school_id = @school_id;

