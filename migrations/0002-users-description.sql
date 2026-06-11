-- 0002: optional free-text user description (admin-only note; NULL = unset). Nullable so
-- "no description" has one representation (NULL), matching the *string in the domain.
ALTER TABLE users ADD COLUMN description TEXT;
