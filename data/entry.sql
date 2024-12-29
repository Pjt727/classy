-- name: ListCourses :many
SELECT * FROM courses;

-- name: UpsertSections :copyfrom
-- INSERT INTO sections 
--     (sections.id, sections.campus, sections.course_id, 
--         sections.school_id, sections.term_year, sections.term_season, 
--         sections.enrollment, sections.max_enrollment, sections.instruction_method,
--         sections.primary_faculty_id, sections.campus)
-- VALUES
--     (@id, @campus, @course_id,
--         @school_id, @term_year, @term_season,
--         @enrollment, @max_enrollment, @instruction_method,
--         @primary_faculty_id, @campus)
-- ON CONFLICT (sections.id, sections.term_year, sections.term_season, sections.course_id, sections.school_id)
-- DO UPDATE SET 
--     sections.enrollment = @enrollment,
--     sections.max_enrollment = @max_enrollment,
--     sections.primary_faculty_id = @primary_faculty_id

-- name: UpsertSections :copyfrom
INSERT INTO sections 
    (id, campus, course_id, 
        school_id, term_year, term_season, 
        enrollment, max_enrollment, instruction_method,
        primary_faculty_id)
VALUES
    ($1, $2, $3,
        $4, $5, $6,
        $7, $8, $9,
        $10);
